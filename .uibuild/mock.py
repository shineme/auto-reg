import io
src = io.open(r'C:\Users\dhl\Desktop\m3\go-protocol\frontend\index.html', encoding='utf-8').read()
mock = """
<script>
window.go = { main: { App: {
  GetConfig: async () => ({ panelBase: "https://web.open-traffic.com", panelKey: "x", panel2Base: "http://web.open-traffic.com", panel2Key: "y",
    mailMode: "hotmail", count: 1000, workers: 2, timezoneId: "Asia/Shanghai", signinAfterRegister: true,
    hotmailPool: "/opt/minimax/hotmail-pool.json", hotmailPreflightWorkers: 8, proxyStyle: "711", proxyUrl: "socks5://u:p@global.rotgb.trustsource.live:10000",
    proxyRegionRotate: "JP,US,GB", proxySessTime: 15, proxySticky: true, proxySubscriptionUrl: "https://subs.2519885.dpdns.org/c/?token=ce7b0ad8f20c084448d03891f892d848",
    proxyPoolWorkers: 16, proxyLocalPorts: "18001-18422", proxyPoolUse: true, ticketMode: "cdp", chromePath: "/usr/bin/google-chrome",
    outputDir: "/opt/minimax/artifacts", signatureSalt: "I*7Cf%WZ#S&%1RlZJ&C2", yySuffix: "ooui", recaptchaSiteKey: "6Lc0gOUtAAAAAM6ovsGBJbA4K1La5ZAOA9B7X39k" }),
  GetHotmailStats: async () => ({ total: 1000, available: 447, used: 553, skipped: 0, reserved: 0 }),
  GetProxyPoolStats: async () => ({ total: 422, alive: 282, dead: 140, checked: 422 }),
  GetProgress: async () => ({ target: 1000, succeeded: 1240, failedAttempts: 1091, currentStage: "等待面板绑定", currentEmail: "drewfasthorse01639@hotmail.com", running: true,
    results: Array.from({length: 12}, function(_, i) { return { email: "user" + i + "@hotmail.com", providerName: "MiniMax" + (997000 + i), userCode: "M.C561_BAY.0.U.MsaArtifacts.-Cu086H", signInId: "9e5f94bc-e8a4-4e73-b8be", createdAt: "2026-11-20T13:" + (10 + i) + ":02Z" }; }) }),
  MailListOptions: async () => ["a@hotmail.com", "b@hotmail.com"]
}}};
</script>
</body>"""
out = src.replace("</body>", mock, 1)
io.open(r'C:\Users\dhl\Desktop\m3\.uibuild\mock.html', 'w', encoding='utf-8', newline='\n').write(out)
print("ok")
