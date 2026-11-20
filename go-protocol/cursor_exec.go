package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// cursorCaptchaWaitLimit 人机验证最长等待时间。
// 服务器（无桌面）默认 90 秒就放弃该账号换出口；本地人工点击可用 MINIMAX_CURSOR_MANUAL=1 无限等待。
func cursorCaptchaWaitLimit() time.Duration {
	if cursorManualHuman {
		return 24 * time.Hour
	}
	if v := os.Getenv("MINIMAX_CURSOR_CAPTCHA_WAIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 90 * time.Second
}

// Execute 跑完整 Cursor 授权流程：deep-control 页面、邮箱登录页、验证码、密码，面板落库。
func (f *CursorFlow) Execute(ctx context.Context, email string, fetchCode func(ctx context.Context, notBefore time.Time) (string, error), onStep func(string)) (*CursorResult, error) {
	signIn, err := f.Panel.StartPluginSignInProvider(ctx, cursorProviderID)
	if err != nil {
		return nil, fmt.Errorf("申请 Cursor 授权失败: %w", err)
	}
	cursorDebug("signIn id=%s url=%s", signIn.ID, signIn.URL)
	if onStep != nil {
		onStep("申请面板授权")
	}
	before, _ := f.Panel.ListCursorUsers(ctx)

	browser, err := launchCursorBrowser(ctx, f.ChromePath, f.ProxyBidge, f.ProfileDir, signIn.URL)
	if err != nil {
		return nil, err
	}
	browser.send("Runtime.enable", nil)
	browser.send("Network.enable", nil)
	defer browser.close()
	if _, err := browser.evalJS(ctx, cursorUIHelperJS, false); err != nil {
		return nil, fmt.Errorf("注入页面脚本失败: %w", err)
	}

	deadline := time.Now().Add(12 * time.Minute)
	codeRequestedAt := time.Now()
	emailTries := 0
	codeTries := 0
	passwordTries := 0
	switchTries := 0
	humanHinted := false
	lastHumanHint := time.Time{}
	humanSince := time.Time{}
	approveTries := 0
	linkTries := 0
	confirmLink := ""
	verifyCode := ""
	hydrated := false

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		st, err := browser.cursorUIState(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		cursorDebug("state host=%s deep=%v email=%v code=%v pwd=%v btns=%v inputs=%v cta=%v text=%v", st.Host, st.DeepControl, st.HasEmail, st.HasCode, st.HasPassword, st.Buttons, st.Inputs, st.CTA, st.Text)
		if !hydrated {
			if len(st.Buttons) == 0 {
				time.Sleep(2 * time.Second)
				continue
			}
			hydrated = true
		}
		// Cursor CLI 会发一封“确认变更账号”邮件，点信里的链接才算登录完成。
		if strings.Contains(st.Text, "Confirm a change to your Cursor account") ||
			strings.Contains(st.Text, "Check your email on this device") ||
			strings.Contains(st.Text, "Waiting for you to confirm") {
			if confirmLink == "" && f.FetchLink != nil && linkTries < 3 {
				linkTries++
				if onStep != nil {
					onStep("等待邮箱里的账号确认邮件")
				}
				link, lerr := f.FetchLink(ctx, time.Now().Add(-10*time.Minute))
				if lerr == nil && link != "" {
					confirmLink = link
				} else {
					cursorDebug("confirm link fetch failed: %v", lerr)
				}
			}
			if confirmLink != "" {
				if onStep != nil {
					onStep("打开确认链接完成登录")
				}
				cursorDebug("open confirm link")
				if okc, nerr := browser.openConfirmTab(ctx, confirmLink); nerr != nil {
					cursorDebug("open tab failed: %v", nerr)
				} else if okc {
					cursorDebug("confirm action clicked")
				}
				confirmLink = ""
				time.Sleep(4 * time.Second)
				continue
			}
			time.Sleep(3 * time.Second)
			continue
		}

		// 面板 sign-in 记录变 done 就收工（账号早就绑过时也能认出来）。
		if state, user, err := f.Panel.CursorSignInState(ctx, signIn.ID); err == nil {
			if state == "done" && user != "" {
				return &CursorResult{SignInID: signIn.ID, User: user, Email: email}, nil
			}
			if state == "error" || state == "expired" || state == "failed" {
				return nil, fmt.Errorf("面板 sign-in 终态异常: %s", state)
			}
		}
		// 面板已落库就收工
		if users, err := f.Panel.ListCursorUsers(ctx); err == nil {
			for _, u := range users {
				found := false
				for _, old := range before {
					if old == u {
						found = true
						break
					}
				}
				if !found {
					return &CursorResult{SignInID: signIn.ID, User: u, Email: email}, nil
				}
			}
		}

		// deep-control 首页：点 Continue to sign in 进入登录页
		if st.DeepControl {
			clicked, _ := browser.clickUI(ctx, "continue to sign in")
			if clicked != "" {
				if onStep != nil {
					onStep("进入 Cursor 登录页")
				}
				time.Sleep(3 * time.Second)
				continue
			}
		}

		// 登录方式页：优先切到验证码登录（邮箱取码），避免密码提交被风控
		if strings.Contains(st.Host, "authenticator") && !st.HasCode && switchTries < 3 {
			cursorDebug("switch attempt btns=%v", st.Buttons)
			clicked, _ := browser.clickSel(ctx, `button[value="magic-code"]`)
			if clicked == "" {
				clicked, _ = browser.clickUI(ctx, "验证码", "verification code", "邮箱登录")
			}
			if clicked != "" {
				switchTries++
				cursorDebug("switched login mode: %s", clicked)
				time.Sleep(3 * time.Second)
				continue
			}
		}
				// 人机验证：程序不代过验证，由用户在浏览器窗口自行点击完成。
		if cursorHumanCheckText(st.Text) {
			if humanSince.IsZero() {
				humanSince = time.Now()
			}
			// 服务器上没有桌面可点，超时就放弃该账号，换出口继续下一个。
			if !cursorManualHuman && time.Since(humanSince) > cursorCaptchaWaitLimit() {
				return nil, fmt.Errorf("人机验证等待超时（%.0fs，服务器端无法人工点击），已跳过该账号",
					cursorCaptchaWaitLimit().Seconds())
			}
			if !humanHinted || time.Since(lastHumanHint) > 15*time.Second {
				lastHumanHint = time.Now()
				humanHinted = true
				if onStep != nil {
					onStep("请在浏览器窗口手动完成人机验证，完成后自动继续")
				}
				fmt.Println("[cursor] 请在 Chrome 窗口点击人机验证（窗口已置于屏幕左上角）")
				if browser.cmd != nil && browser.cmd.Process != nil {
					focusCursorWindow(uint32(browser.cmd.Process.Pid))
				}
			}
			time.Sleep(2 * time.Second)
			continue
		}

		// Cursor 的一次性引导页：点 Continue 过去。
		if strings.Contains(st.Text, "Before you Start Building") || strings.Contains(st.Text, "Connect your repos") || strings.Contains(st.Text, "finish setup") {
			if ok, _ := browser.clickAtReal(ctx, "Continue"); ok {
				if onStep != nil {
					onStep("完成 Cursor 引导页")
				}
				cursorDebug("onboarding continue clicked")
				time.Sleep(3 * time.Second)
				continue
			}
		}

		// Cookie 提示条：先关掉，免得挡住后面的按钮。
		if clicked, _ := browser.clickOut(ctx, "Dismiss cookie notice"); clicked != "" {
			cursorDebug("cookie notice dismissed")
			time.Sleep(1 * time.Second)
			continue
		}
		// 授权确认页：点 Sign in 完成授权。
		if strings.Contains(strings.ToLower(st.URL), "logindeepcontrol") && approveTries < 12 {
			approveTries++
			if ok, _ := browser.clickAtReal(ctx, "Sign in"); ok {
				if onStep != nil {
					onStep("确认授权")
				}
				cursorDebug("approved sign-in request")
				time.Sleep(3 * time.Second)
				continue
			}
			// 引导清单会盖住 Sign in 按钮，先把它关掉。
			if strings.Contains(st.Text, "Connect your repos") || strings.Contains(st.Text, "finish setup") ||
				strings.Contains(st.Text, "Build cloud-based code automations") || strings.Contains(st.Text, "Maybe later") {
				if ok, _ := browser.clickAtReal(ctx, "Maybe later"); ok {
					cursorDebug("closed onboarding overlay")
					time.Sleep(2 * time.Second)
					continue
				}
				if clicked, _ := browser.clickUI(ctx, "Maybe later"); clicked != "" {
					cursorDebug("closed onboarding overlay via js click")
					time.Sleep(2 * time.Second)
					continue
				}
			}
			// 真实鼠标点击不生效时，用 JS 点击兜底。
			if approveTries >= 3 {
				if clicked, _ := browser.clickUI(ctx, "Sign in"); clicked != "" {
					cursorDebug("approved sign-in request via js click")
					time.Sleep(3 * time.Second)
					continue
				}
				time.Sleep(2 * time.Second)
			}
		}

// 密码页：新账号需要设密码
		if st.HasPassword && passwordTries < 3 && !strings.Contains(st.URL, "/magic-code") && cursorPageWantsSignup(st) {
			pwd := f.Password
			if pwd == "" {
				pwd = randomPassword()
			}
			if st.HasName {
				_, _ = browser.setUI(ctx, "name", cursorDisplayName(email))
			}
			ok, err := browser.setUIAll(ctx, "password", pwd)
			if err != nil {
				return nil, err
			}
			if ok {
				if onStep != nil {
					onStep("设置账号密码")
				}
				clicked, _ := browser.clickUI(ctx, "create account", "sign up", "创建账号", "注册", "continue", "继续", "登录")
				if clicked == "" {
					clicked, _ = browser.submitUI(ctx)
				}
				if clicked != "" {
					passwordTries++
					time.Sleep(3 * time.Second)
					continue
				}
			}
		}
		// 验证码页
		if st.HasCode && codeTries < 3 {
			if verifyCode == "" {
				if onStep != nil {
					onStep("轮询邮箱验证码")
				}
				code, err := fetchCode(ctx, codeRequestedAt.Add(-3*time.Minute))
				if err != nil {
					return nil, fmt.Errorf("取 Cursor 验证码失败: %w", err)
				}
				verifyCode = code
			}
			clicked, err := browser.fillCode(ctx, verifyCode)
			if err != nil {
				return nil, err
			}
			if onStep != nil {
				onStep("填写验证码")
			}
			cursorDebug("code submitted via %q", clicked)
			codeTries++
			if onStep != nil {
				onStep("提交登录")
			}
			time.Sleep(4 * time.Second)
			continue
		}

		// 邮箱页
		if st.HasEmail && !st.HasPassword && !st.HasCode && emailTries < 3 {
			ok, err := browser.setUI(ctx, "email", email)
			if err != nil {
				return nil, err
			}
			if ok {
				if onStep != nil {
					onStep("填写邮箱")
				}
				time.Sleep(500 * time.Millisecond)
				clicked, _ := browser.clickUI(ctx, cursorEmailSubmitWords...)
				if clicked == "" {
					clicked, _ = browser.submitUI(ctx)
				}
				if clicked != "" {
					emailTries++
					codeRequestedAt = time.Now()
					if onStep != nil {
						onStep("已提交邮箱，等待验证码")
					}
					time.Sleep(3 * time.Second)
					continue
				}
			}
		}

		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("Cursor 授权超时未完成")
}

// cursorPageWantsSignup 判断密码页是不是创建账号模式（已有账号登录时不该设密码）。
// cursorHumanCheckText 判断页面上是不是人机验证提示（中英文都要认）。
func cursorHumanCheckText(text string) bool {
	marks := []string{
		"sure you are human",
		"verify the user is human",
		"confirm you are human",
		"verify that you are human",
		"确认您是真人",
		"确认你是真人",
		"验证您是人类",
		"验证你是人类",
		"人机验证",
	}
	low := strings.ToLower(text)
	for _, m := range marks {
		if strings.Contains(low, strings.ToLower(m)) {
			return true
		}
	}
	return false
}

func cursorPageWantsSignup(st *cursorUIState) bool {
	for _, c := range st.CTA {
		l := strings.ToLower(c)
		if strings.Contains(l, "create account") || strings.Contains(l, "sign up") ||
			strings.Contains(l, "创建账号") || strings.Contains(l, "注册") {
			return true
		}
	}
	return false
}
