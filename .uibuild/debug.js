setTimeout(function () {
  var t = document.querySelector(".aside table");
  var w = document.querySelector(".table-wrap");
  var a = document.querySelector(".aside");
  var out = {
    aside: a.clientWidth,
    wrap: w.clientWidth,
    table: t.getBoundingClientRect().width,
    scrollW: t.scrollWidth,
    cols: Array.prototype.map.call(t.querySelectorAll("tbody tr:first-child td"), function (td) { return Math.round(td.getBoundingClientRect().width); }),
    cssAsideVar: getComputedStyle(document.documentElement).getPropertyValue("--aside-w"),
    layout: getComputedStyle(t).tableLayout
  };
  var d = document.createElement("div");
  d.id = "debugout";
  d.textContent = JSON.stringify(out);
  document.body.appendChild(d);
}, 1200);
