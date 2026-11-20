setTimeout(function () {
  var row = document.querySelector(".quick-row");
  var sc = document.getElementById("quickPicks");
  var btn = document.getElementById("btnRandomMail");
  var d = document.createElement("div");
  d.id = "dbg2";
  d.textContent = JSON.stringify({
    rowW: row ? Math.round(row.getBoundingClientRect().width) : -1,
    rowRight: row ? Math.round(row.getBoundingClientRect().right) : -1,
    scW: sc ? Math.round(sc.getBoundingClientRect().width) : -1,
    scRight: sc ? Math.round(sc.getBoundingClientRect().right) : -1,
    btnW: btn ? Math.round(btn.getBoundingClientRect().width) : -1,
    btnRight: btn ? Math.round(btn.getBoundingClientRect().right) : -1,
    btnDisplay: btn ? getComputedStyle(btn).display : ""
  });
  document.body.appendChild(d);
}, 1700);
