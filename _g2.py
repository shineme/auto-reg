import json, re, urllib.request
tok = open(r'C:\Users\dhl\Desktop\m3\gtok.txt').read()
u = 'https://graph.microsoft.com/v1.0/me/messages?$top=12&$select=subject,body,receivedDateTime,from&$orderby=receivedDateTime%20desc'
req = urllib.request.Request(u, headers={'Authorization': 'Bearer ' + tok})
msgs = json.loads(urllib.request.urlopen(req, timeout=30).read())['value']
for m in msgs:
    body = (m.get('body') or {}).get('content') or ''
    links = re.findall(r'https?://[^\s"\'<>)\\]+', m.get('bodyPreview','') + ' ' + body)
    print(m['receivedDateTime'], '|', m['from']['emailAddress']['address'], '|', m['subject'])
    for l in links[:6]:
        print('    ', l[:150])
