x = "password"
y = "Password"
z = "passwd"
open("redact-test.txt","w",encoding="utf-8").write(x+"|"+y+"|"+z)
