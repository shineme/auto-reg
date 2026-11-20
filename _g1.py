import json, sys, urllib.parse, urllib.request
P = r'C:\Users\dhl\Desktop\m3\artifacts\go-protocol\hotmail-pool-1118.json'
email = sys.argv[1]
pool = json.load(open(P, encoding='utf-8'))
a = [x for x in pool['accounts'] if x['email'] == email][0]
d = urllib.parse.urlencode({'client_id': a['clientId'], 'refresh_token': a['refreshToken'], 'grant_type': 'refresh_token', 'scope': 'https://graph.microsoft.com/Mail.ReadWrite offline_access'}).encode()
req = urllib.request.Request('https://login.microsoftonline.com/consumers/oauth2/v2.0/token', data=d)
tok = json.loads(urllib.request.urlopen(req, timeout=30).read())
open(r'C:\Users\dhl\Desktop\m3\gtok.txt','w').write(tok['access_token'])
print('token ok')
