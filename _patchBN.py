import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\frontend\\assets\\app.js'
s = io.open(p, encoding='utf-8').read()
anchor = '''      } else if (id === "import-pool") {'''
new = '''      } else if (id === "cursor-bind") {
        await app().SaveConfig(cfg);
        await app().StartCursorBinding(cfg, cfg.count || 1);
        toast("Cursor 绑定已启动", "ok");
      } else if (id === "cursor-stop") {
        await app().StopCursorBinding();
        toast("已发送停止信号");
      } else if (id === "import-pool") {'''
assert anchor in s
io.open(p,'w',encoding='utf-8',newline='').write(s.replace(anchor, new, 1))
print('ok')
