package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// cursorUIState 是 Cursor 页面（deep-control / authenticator）的快照。
type cursorUIState struct {
	URL         string   `json:"url"`
	Host        string   `json:"host"`
	HasEmail    bool     `json:"hasEmail"`
	HasCode     bool     `json:"hasCode"`
	HasPassword bool     `json:"hasPassword"`
	HasName     bool     `json:"hasName"`
	Buttons     []string `json:"buttons"`
	BtnsHTML   []string `json:"btnsHtml"`
	CTA        []string `json:"cta"`
	Inputs      []string `json:"inputs"`
	Text        string   `json:"text"`
	DeepControl bool     `json:"deepControl"`
	SignedIn    bool     `json:"signedIn"`
}

// cursorUIHelperJS 注入页面，负责找输入框、点按钮、读状态。
const cursorUIHelperJS = `
(function () {
  function setNativeValue(el, value) {
    var proto = (el.tagName === 'TEXTAREA') ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    var desc = Object.getOwnPropertyDescriptor(proto, 'value');
    if (desc && desc.set) { desc.set.call(el, value); }
    else { el.value = value; }
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }
  function visible(el) {
    if (!el) { return false; }
    if (el.disabled) { return false; }
    var r = el.getBoundingClientRect();
    if (r.width <= 0 && r.height <= 0) { return false; }
    var st = window.getComputedStyle(el);
    return st.visibility !== 'hidden' && st.display !== 'none' && st.opacity !== '0';
  }
  function firstVisible(list) {
    for (var i = 0; i < list.length; i++) { if (visible(list[i])) { return list[i]; } }
    return null;
  }
  function pick(kind) {
    var sels;
    if (kind === 'email') {
      sels = ['input[name=email]', 'input[type=email]', 'input[id*=email i]', 'input[autocomplete=email]', 'input[placeholder*=mail i]'];
    } else if (kind === 'code') {
      sels = ['input[autocomplete=one-time-code]', 'input[name=code]', 'input[id*=code i]', 'input[placeholder*=code i]', 'input[name*=otp i]', 'input[inputmode=numeric]', "input[maxlength='6']"];
    } else if (kind === 'password') {
      sels = ['input[type=password]', 'input[name=password]', 'input[id*=password i]'];
    } else {
      sels = ['input[name=name]', 'input[id*=name i]', 'input[autocomplete=name]', 'input[placeholder*=name i]'];
    }
    for (var i = 0; i < sels.length; i++) {
      var el = firstVisible(Array.prototype.slice.call(document.querySelectorAll(sels[i])));
      if (el) { return el; }
    }
    if (kind === 'code') {
      var fb = document.querySelector('input[name=code]');
      if (fb) { return fb; }
    }
    return null;
  }
  window.__mmSet = function (kind, value) {
    var el = pick(kind);
    if (!el) { return false; }
    el.focus();
    setNativeValue(el, value);
    return true;
  };
  window.__mmClick = function (words) {
    var all = Array.prototype.slice.call(document.querySelectorAll('button, a[role=button], [role=button], input[type=submit]'));
    for (var w = 0; w < words.length; w++) {
      var word = words[w].toLowerCase();
      for (var i = 0; i < all.length; i++) {
        var el = all[i];
        if (!visible(el)) { continue; }
        var t = (el.innerText || el.value || el.textContent || '').trim().toLowerCase();
        if (t === word) { el.click(); return t; }
      }
    }
    for (var w2 = 0; w2 < words.length; w2++) {
      var sub = words[w2].toLowerCase();
      for (var i2 = 0; i2 < all.length; i2++) {
        var el2 = all[i2];
        if (!visible(el2)) { continue; }
        var t2 = (el2.innerText || el2.value || el2.textContent || '').trim().toLowerCase();
        if (t2 && t2.indexOf(sub) >= 0) { el2.click(); return t2; }
      }
    }
    return '';
  };
  window.__mmFillCode = function (value) {
    var digits = String(value).split('');
    var el = document.querySelector('input[name=code]');
    if (el) { el.focus(); setNativeValue(el, value); }
    var boxes = Array.prototype.slice.call(document.querySelectorAll('input[type=text]'))
      .filter(function (i) { return visible(i) && !i.name; });
    for (var i = 0; i < boxes.length && i < digits.length; i++) {
      setNativeValue(boxes[i], digits[i]);
    }
    if (el) { return true; }
    return boxes.length > 0;
  };
  window.__mmClickOut = function (text) {
    var want = String(text).trim().toLowerCase();
    var all = Array.prototype.slice.call(document.querySelectorAll('button, a[role=button], [role=button]'));
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (!visible(el)) { continue; }
      if (el.closest && el.closest('#onetrust-consent-sdk, .onetrust-pc-dark-filter, #ot-sdk-container')) { continue; }
      var t = (el.innerText || el.getAttribute('aria-label') || el.value || el.title || el.textContent || '').trim();
      if (t.toLowerCase() === want) { el.click(); return t; }
    }
    return '';
  };
  window.__mmRect = function (text) {
    var want = String(text).trim().toLowerCase();
    var all = Array.prototype.slice.call(document.querySelectorAll('button, a[role=button], [role=button]'));
    // OneTrust 的同意框和全屏遮罩会挡住按钮，先让它们别挡路。
    var overlays = Array.prototype.slice.call(document.querySelectorAll(
      '#onetrust-consent-sdk, #ot-sdk-container, .onetrust-pc-dark-filter, .onetrust-pc-dark-overlay'));
    for (var oi = 0; oi < overlays.length; oi++) {
      try {
        overlays[oi].style.pointerEvents = 'none';
        overlays[oi].style.display = 'none';
      } catch (e) {}
    }
    for (var i = 0; i < all.length; i++) {
      var el = all[i];
      if (!visible(el)) { continue; }
      if (el.closest && el.closest('#onetrust-consent-sdk, #ot-sdk-container')) { continue; }
      var t = (el.innerText || el.getAttribute('aria-label') || el.value || el.title || '').trim();
      if (t.toLowerCase() === want) {
        try { el.scrollIntoView({ block: 'center', inline: 'center' }); } catch (e) {}
        var r = el.getBoundingClientRect();
        var cx = r.x + r.width / 2, cy = r.y + r.height / 2;
        var top = document.elementFromPoint(cx, cy);
        if (top && top !== el && !el.contains(top)) {
          var topInOverlay = top.closest && top.closest(
            '#onetrust-consent-sdk, #ot-sdk-container, .onetrust-pc-dark-filter, .onetrust-pc-dark-overlay');
          if (topInOverlay) {
            topInOverlay.style.pointerEvents = 'none';
            topInOverlay.style.display = 'none';
            top = document.elementFromPoint(cx, cy);
          }
        }
        if (top && top !== el && !el.contains(top)) {
          return { err: 'covered', x: cx, y: cy, top: top.tagName + '|' + (top.innerText || '').slice(0, 40) };
        }
        return { x: cx, y: cy };
      }
    }
    return { err: 'notfound' };
  };
  window.__mmSetAll = function (kind, value) {
    var sels = ['input[type=password]'];
    var n = 0;
    for (var i = 0; i < sels.length; i++) {
      var list = Array.prototype.slice.call(document.querySelectorAll(sels[i]));
      for (var j = 0; j < list.length; j++) {
        if (visible(list[j])) { setNativeValue(list[j], value); n++; }
      }
    }
    return n > 0;
  };
  window.__mmClickSel = function (sel) {
    var el = document.querySelector(sel);
    if (!el) { return ''; }
    el.click();
    return (el.innerText || el.value || '').trim();
  };
  window.__mmSubmit = function () {
    var all = Array.prototype.slice.call(document.querySelectorAll('button, input[type=submit]'))
      .filter(function (b) { return visible(b); })
      .filter(function (b) {
        var t = (b.innerText || b.value || '').trim().toLowerCase();
        return t && t.indexOf('cancel') < 0 && t.indexOf('取消') < 0 && t.indexOf('resend') < 0 && t.indexOf('重发') < 0;
      });
    if (!all.length) { return ''; }
    var last = all[all.length - 1];
    last.click();
    return (last.innerText || last.value || '').trim();
  };
  window.__mmState = function () {
    var email = pick('email');
    var code = pick('code');
    var pwd = pick('password');
    var name = pick('name');
    var btns = Array.prototype.slice.call(document.querySelectorAll('button'))
      .map(function (b) { return (b.innerText || '').trim(); })
      .filter(function (t) { return t.length > 0; });
    var body = (document.body && document.body.innerText) || '';
    var deep = false;
    for (var i = 0; i < btns.length; i++) {
      if (btns[i].toLowerCase().indexOf('continue to sign in') >= 0) { deep = true; }
    }
    return {
      url: location.href,
      host: location.hostname,
      hasEmail: !!email,
      hasCode: !!code,
      hasPassword: !!pwd,
      hasName: !!name,
      buttons: btns.slice(0, 20),
      cta: Array.prototype.slice.call(document.querySelectorAll('button,[role=button],a,input[type=submit]')).map(function (e) { return [e.tagName, (e.innerText || e.getAttribute('aria-label') || e.value || '').trim().slice(0, 24), e.getAttribute('name') || '', e.getAttribute('value') || '', e.getAttribute('data-method') || ''].join('~'); }),
      inputs: Array.prototype.slice.call(document.querySelectorAll('input')).map(function (i) { return [i.type, i.name, i.id, i.placeholder, i.disabled ? 'disabled' : 'on', Math.round(i.getBoundingClientRect().width), (i.value || '').length].join('|'); }),
      text: body.slice(0, 2000),
      deepControl: deep,
      signedIn: /you'?re signed in|signed in to cursor|sign-in (is )?complete|授权成功|已登录/i.test(body)
    };
  };
})();
`

// cursorUIStateOf 读页面状态。
func (b *cursorBrowser) cursorUIState(ctx context.Context) (*cursorUIState, error) {
	href, _ := b.evalJS(ctx, "location.href", false)
	kind, _ := b.evalJS(ctx, "typeof window.__mmState", false)
	cursorDebug("href=%v typeof=%v", href, kind)
	v, err := b.evalJS(ctx, "window.__mmState ? window.__mmState() : null", false)
	if err != nil {
		cursorDebug("eval err=%v", err)
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		cursorDebug("raw=%v", v)
		if _, ierr := b.evalJS(ctx, cursorUIHelperJS, false); ierr == nil {
			return nil, fmt.Errorf("页面脚本已重新注入，稍后重试")
		}
		return nil, fmt.Errorf("页面状态读取失败")
	}
	raw, _ := json.Marshal(m)
	var st cursorUIState
	_ = json.Unmarshal(raw, &st)
	return &st, nil
}

// setUI 往页面里填值。
func (b *cursorBrowser) setUI(ctx context.Context, kind, value string) (bool, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmSet(%q, %q)", kind, value), false)
	if err != nil {
		return false, err
	}
	ok, _ := v.(bool)
	return ok, nil
}

// clickUI 按关键词点按钮。
func (b *cursorBrowser) clickUI(ctx context.Context, words ...string) (string, error) {
	list, _ := json.Marshal(words)
	v, err := b.evalJS(ctx, "window.__mmClick("+string(list)+")", false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// setUIAll 往页面里所有同类输入框填值（密码 + 确认密码）。
func (b *cursorBrowser) setUIAll(ctx context.Context, kind, value string) (bool, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmSetAll(%q, %q)", kind, value), false)
	if err != nil {
		return false, err
	}
	ok, _ := v.(bool)
	return ok, nil
}

// clickSel 按 CSS 选择器直接点击元素。
func (b *cursorBrowser) clickSel(ctx context.Context, sel string) (string, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmClickSel(%q)", sel), false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// submitUI 点最后一个可见按钮（表单提交）。
func (b *cursorBrowser) submitUI(ctx context.Context) (string, error) {
	v, err := b.evalJS(ctx, "window.__mmSubmit ? window.__mmSubmit() : ''", false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// cursorEmailSubmitWords 是在邮箱页点继续的候选文案（中英文都覆盖）。
var cursorEmailSubmitWords = []string{
	"continue with email", "continue with e-mail", "continue",
	"使用电子邮件继续", "继续", "使用电子邮件",
	"sign in", "sign up", "next", "下一步",
	"发送验证码", "send code",
}

// cursorCodeSubmitWords 是在验证码页点确认的候选文案。
var cursorCodeSubmitWords = []string{
	"verify", "continue", "submit", "confirm",
	"登录", "验证", "确认", "继续", "提交",
}

// cursorDisplayName 用邮箱前缀生成一个显示名。
func cursorDisplayName(email string) string {
	name := email
	if i := strings.Index(name, "@"); i > 0 {
		name = name[:i]
	}
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	if len(name) > 24 {
		name = name[:24]
	}
	return name
}
// cursorDebug 输出调试日志（MINIMAX_CURSOR_DEBUG=1 时开启）。
func cursorDebug(format string, args ...any) {
	if os.Getenv("MINIMAX_CURSOR_DEBUG") != "1" {
		return
	}
	fmt.Printf("[cursor-debug] "+format+"\n", args...)
}

// cursorCDPEvent 记录浏览器控制台错误和网络失败，调试用。
func cursorCDPEvent(method string, data []byte) {
	if os.Getenv("MINIMAX_CURSOR_CDP") != "1" {
		return
	}
	switch method {
	case "Runtime.consoleAPICalled", "Runtime.exceptionThrown", "Network.loadingFailed", "Network.responseReceived":
		if len(data) > 600 {
			data = data[:600]
		}
		fmt.Printf("[cursor-cdp] %s %s\n", method, string(data))
	}
}

// fillCode 填入验证码：写入 OTP 输入框后发真实回车事件，再尝试点提交按钮。
func (b *cursorBrowser) fillCode(ctx context.Context, code string) (string, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmFillCode(%q)", code), false)
	if err != nil {
		return "", err
	}
	ok, _ := v.(bool)
	if !ok {
		return "", fmt.Errorf("找不到验证码输入框")
	}
	enterText := string(rune(13))
	for _, typ := range []string{"keyDown", "char", "keyUp"} {
		payload := map[string]any{
			"type": typ, "key": "Enter", "code": "Enter",
			"windowsVirtualKeyCode": 13, "nativeVirtualKeyCode": 13,
		}
		if typ == "char" {
			payload["text"] = enterText
		}
		_, _ = b.sendWait(ctx, "Input.dispatchKeyEvent", payload)
	}
	time.Sleep(1500 * time.Millisecond)
	clicked, _ := b.clickUI(ctx, cursorCodeSubmitWords...)
	if clicked == "" {
		// 没有明确提交按钮就再回车一次，避免误点“重新发送”。
		for _, typ := range []string{"keyDown", "char", "keyUp"} {
			payload := map[string]any{
				"type": typ, "key": "Enter", "code": "Enter",
				"windowsVirtualKeyCode": 13, "nativeVirtualKeyCode": 13,
			}
			if typ == "char" {
				payload["text"] = enterText
			}
			_, _ = b.sendWait(ctx, "Input.dispatchKeyEvent", payload)
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return clicked, nil
}

// clickOut 点按文案完全一致的按钮，跳过 Cookie 同意框里的元素。
func (b *cursorBrowser) clickOut(ctx context.Context, text string) (string, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmClickOut(%q)", text), false)
	if err != nil {
		return "", err
	}
	s, _ := v.(string)
	return s, nil
}

// clickAtReal 用真实鼠标事件点按文案完全一致的按钮（部分页面只认 trusted 事件）。
func (b *cursorBrowser) clickAtReal(ctx context.Context, text string) (bool, error) {
	v, err := b.evalJS(ctx, fmt.Sprintf("window.__mmRect(%q)", text), false)
	if err != nil {
		return false, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		cursorDebug("clickAtReal %q: 页面未返回坐标", text)
		return false, nil
	}
	if reason, _ := m["err"].(string); reason != "" {
		cursorDebug("clickAtReal %q 失败: %s %v", text, reason, m["top"])
		return false, nil
	}
	x, _ := m["x"].(float64)
	y, _ := m["y"].(float64)
	if x == 0 && y == 0 {
		cursorDebug("clickAtReal %q 失败: 坐标为 0", text)
		return false, nil
	}
	for _, typ := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		button := "left"
		if typ == "mouseMoved" {
			button = "none"
		}
		payload := map[string]any{
			"type": typ, "x": x, "y": y, "button": button, "clickCount": 1,
		}
		if typ != "mouseMoved" {
			payload["buttons"] = 1
		}
		_, _ = b.sendWait(ctx, "Input.dispatchMouseEvent", payload)
	}
	return true, nil
}

// openNewTab 在同一个浏览器里新开一个标签页打开链接（确认链接必须留在原浏览器里）。
func (b *cursorBrowser) openNewTab(ctx context.Context, url string) error {
	if b.bws == nil {
		return fmt.Errorf("浏览器级连接不可用")
	}
	b.bmsgID++
	id := b.bmsgID
	payload, err := json.Marshal(map[string]any{
		"id": id, "method": "Target.createTarget", "params": map[string]any{"url": url},
	})
	if err != nil {
		return err
	}
	_ = b.bws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.bws.WriteMessage(websocket.TextMessage, payload); err != nil {
		return err
	}
	return nil
}

// browserSend 发一条浏览器级 CDP 命令并等它的结果。
func (b *cursorBrowser) browserSend(ctx context.Context, method string, params map[string]any, sessionID string) (map[string]any, error) {
	if b.bws == nil {
		return nil, fmt.Errorf("浏览器级连接不可用")
	}
	b.bmsgID++
	id := b.bmsgID
	payload := map[string]any{"id": id, "method": method, "params": params}
	if sessionID != "" {
		payload["sessionId"] = sessionID
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	_ = b.bws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := b.bws.WriteMessage(websocket.TextMessage, data); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_ = b.bws.SetReadDeadline(time.Now().Add(20 * time.Second))
		_, raw, err := b.bws.ReadMessage()
		if err != nil {
			return nil, err
		}
		var msg map[string]any
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		if mid, ok := msg["id"].(float64); ok && int(mid) == id {
			return msg, nil
		}
	}
	return nil, fmt.Errorf("%s 超时", method)
}

// openConfirmTab 在同一浏览器新开标签打开确认链接，并点下页面上的 Confirm。
func (b *cursorBrowser) openConfirmTab(ctx context.Context, url string) (bool, error) {
	res, err := b.browserSend(ctx, "Target.createTarget", map[string]any{"url": url}, "")
	if err != nil {
		return false, err
	}
	result, _ := res["result"].(map[string]any)
	tid, _ := result["targetId"].(string)
	if tid == "" {
		return false, fmt.Errorf("新标签页创建失败")
	}
	ares, err := b.browserSend(ctx, "Target.attachToTarget", map[string]any{"targetId": tid, "flatten": true}, "")
	if err != nil {
		return false, err
	}
	aresult, _ := ares["result"].(map[string]any)
	sid, _ := aresult["sessionId"].(string)
	if sid == "" {
		return false, fmt.Errorf("无法附加到新标签页")
	}
	expr := "(function(){var bs=document.querySelectorAll('button');for(var i=0;i<bs.length;i++){var t=(bs[i].innerText||'').trim().toLowerCase();if(t==='confirm'||t==='确认'){bs[i].click();return true;}}return false;})()"
	for i := 0; i < 12; i++ {
		time.Sleep(1500 * time.Millisecond)
		out, err := b.browserSend(ctx, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true}, sid)
		if err != nil {
			continue
		}
		r2, _ := out["result"].(map[string]any)
		val, _ := r2["result"].(map[string]any)
		if v, _ := val["value"].(bool); v {
			return true, nil
		}
	}
	return false, nil
}
