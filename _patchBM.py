import io
p = r'C:\\Users\\dhl\\Desktop\\m3\\go-protocol\\frontend\\index.html'
s = io.open(p, encoding='utf-8').read()
anchor = '''            <button class="queue-item" data-cmd="fetch-sub">'''
new = '''            <button class="queue-item" data-cmd="cursor-bind">
              <span class="qi-icon">\\u25c9</span>
              <span class="qi-text"><b>绑定 Cursor 账号</b><span>用号池邮箱取码授权</span></span>
            </button>
            <button class="queue-item" data-cmd="cursor-stop">
              <span class="qi-icon">\\u25a0</span>
              <span class="qi-text"><b>停止 Cursor 绑定</b><span>结束当前绑定流程</span></span>
            </button>
'''
assert anchor in s
io.open(p,'w',encoding='utf-8',newline='').write(s.replace(anchor, new + anchor, 1))
print('ok')
