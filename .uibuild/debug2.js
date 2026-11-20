setTimeout(function () {
  var p = document.querySelector(".palette");
  var cs = getComputedStyle(p);
  var d = document.createElement("div");
  d.id = "debugout";
  d.textContent = JSON.stringify({
    bg: cs.backgroundColor, opacity: cs.opacity, z: cs.zIndex, pos: cs.position,
    rect: p.getBoundingClientRect().toJSON(), vis: cs.visibility, display: cs.display,
    filter: cs.backdropFilter, mixBlend: cs.mixBlendMode, isolation: cs.isolation
  });
  document.body.appendChild(d);
}, 1600);
