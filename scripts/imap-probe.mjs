import tls from "node:tls";

const host = process.env.IMAP_HOST || "imap.example.com";
const port = Number(process.env.IMAP_PORT || 993);
const user = process.env.IMAP_USER;
const password = process.env.IMAP_PASSWORD;
const ids = process.env.FETCH_IDS || "239";
const partial = process.env.FETCH_PARTIAL || "0.8192";

const socket = tls.connect({ host, port, servername: host, rejectUnauthorized: true });
socket.setEncoding("binary");
socket.on("data", (chunk) => console.log("DATA", chunk));
socket.on("error", (error) => console.log("ERROR", error.message));

function send(line) {
  console.log("SEND", line.replace(/^LOGIN .*/, "LOGIN [redacted]"));
  socket.write(`${line}\r\n`, "latin1");
}

await new Promise((resolve, reject) => {
  socket.once("secureConnect", resolve);
  socket.once("error", reject);
});
await new Promise((resolve) => setTimeout(resolve, 500));
send(`A1 LOGIN "${user}" "${password}"`);
await new Promise((resolve) => setTimeout(resolve, 1000));
send('A2 SELECT "INBOX"');
await new Promise((resolve) => setTimeout(resolve, 1000));
send(`A3 FETCH ${ids} (BODY.PEEK[HEADER.FIELDS (SUBJECT FROM TO DATE)] BODY.PEEK[TEXT]<${partial}>)`);
await new Promise((resolve) => setTimeout(resolve, 5000));
socket.end();
