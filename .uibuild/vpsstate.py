import paramiko
c = paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect("64.235.39.49", 22, username="root", password="3LskP9pONkBYmLWLsflVMKA1", timeout=20, banner_timeout=30)
def run(cmd, timeout=60):
    i, o, e = c.exec_command(cmd, timeout=timeout)
    out = o.read().decode(errors="replace"); err = e.read().decode(errors="replace")
    if err.strip(): print("[err]", err[:300])
    return out
print("--- services ---")
print(run("systemctl is-active minimax-web singbox-pool 2>&1").strip())
print("--- summary ---")
print(run("cat /opt/minimax/artifacts/summary.json 2>/dev/null | head -c 1200").strip())
print("--- processes ---")
print(run("ps aux | grep -E '[m]inimax-web|[s]ing-box' | awk '{print $2, $3, $11}' | head -8").strip())
c.close()
