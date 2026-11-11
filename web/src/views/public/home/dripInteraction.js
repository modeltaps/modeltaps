// ==============================|| PUBLIC HOME — DRIP INTERACTION ||============================== //
// Lets visitors play with the drip (DripFlow.jsx):
//   - a tap on the surface lets a drop fall right there: a hairline appears above the point, the
//     drop gathers and falls, and the ripple starts where it lands, naming a few of the models it
//     passes;
//   - pressing and holding keeps the drop swelling (up to about 2.2x) until it is let go; a bigger
//     drop throws a wider ripple that reaches more models.
// While someone plays, the CSS loop steps aside (.drip-live); five seconds after the last drop it
// starts again from the top.
//
// The function must stay self-contained (no imports, no module-level references): the prerendered
// static home page inlines its source text (scripts/prerender-home.mjs), and the app calls it from
// DripFlow. It returns a cleanup function. Under reduced motion it does nothing.

export function attachDripInteraction(root) {
  var noop = function () {};
  if (!root || typeof window === 'undefined' || !root.animate) return noop;
  if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return noop;
  var svg = root.querySelector('svg');
  var markEl = root.querySelector('.drip-drop .drip-mark');
  if (!svg || !markEl || !svg.getScreenCTM) return noop;

  // geometry shared with DripFlow.jsx
  var NS = 'http://www.w3.org/2000/svg';
  var CX = 240,
    SY = 236,
    RX = 236,
    K = 0.26,
    TAP_Y = 18,
    H = 31;
  var HOLD_MS = 220,
    IDLE_MS = 5000;
  var MODELS = [
    'claude-opus-5',
    'gpt-5.5',
    'gemini-3-pro',
    'deepseek-v4',
    'qwen3-max',
    'glm-5',
    'kimi-k2.5',
    'llama-4',
    'grok-4',
    'mistral-large-3',
    'claude-sonnet-5',
    'gpt-5.5-mini'
  ];
  var markD = markEl.getAttribute('d');

  var layer = document.createElementNS(NS, 'g');
  svg.appendChild(layer);
  var dots = Array.prototype.map.call(svg.querySelectorAll('.drip-dot'), function (n) {
    var r = parseFloat(n.getAttribute('r'));
    return { node: n, x: parseFloat(n.getAttribute('cx')), y: parseFloat(n.getAttribute('cy')), depth: (r - 0.8) / 1.1 };
  });

  function el(tag, attrs) {
    var n = document.createElementNS(NS, tag);
    for (var k in attrs) n.setAttribute(k, attrs[k]);
    layer.appendChild(n);
    return n;
  }
  function tf(x, top, sx, sy) {
    return 'translate(' + (x - 12.8).toFixed(1) + 'px,' + top.toFixed(1) + 'px) scale(' + sx.toFixed(3) + ',' + sy.toFixed(3) + ')';
  }
  function toSurface(evt) {
    var pt = svg.createSVGPoint();
    pt.x = evt.clientX;
    pt.y = evt.clientY;
    var p = pt.matrixTransform(svg.getScreenCTM().inverse());
    var u = (p.x - CX) / RX,
      v = (p.y - SY) / (RX * K),
      m = Math.sqrt(u * u + v * v),
      lim = 0.9;
    if (m > lim) {
      u *= lim / m;
      v *= lim / m;
    }
    return { x: CX + u * RX, y: SY + v * RX * K };
  }

  // the CSS loop steps aside while drops are in flight, and comes back after a quiet spell
  var inFlight = 0,
    idleTimer = 0;
  function goLive() {
    clearTimeout(idleTimer);
    root.classList.add('drip-live');
  }
  function settle() {
    clearTimeout(idleTimer);
    idleTimer = setTimeout(function () {
      if (!inFlight) root.classList.remove('drip-live');
    }, IDLE_MS);
  }

  function splash(p, strength) {
    var R = 250 * strength,
      dur = 1600 * Math.sqrt(strength);
    [0, 220, 480].forEach(function (delay, i) {
      var e = el('ellipse', {
        class: 'drip-live-ripple',
        cx: p.x.toFixed(1),
        cy: p.y.toFixed(1),
        rx: R.toFixed(1),
        ry: (R * K).toFixed(1)
      });
      e.style.transformBox = 'fill-box';
      e.style.transformOrigin = 'center';
      e.animate(
        [
          { transform: 'scale(0.02)', opacity: 0.8 - i * 0.15 },
          { transform: 'scale(1)', opacity: 0 }
        ],
        { duration: dur, delay: delay, fill: 'both' }
      ).finished.then(
        function () {
          e.remove();
        },
        function () {}
      );
    });
    [
      [-10, 1.6],
      [12, 1.3],
      [2, 1.1]
    ].forEach(function (s) {
      var c = el('circle', {
        class: 'drip-live-spray',
        cx: p.x.toFixed(1),
        cy: (p.y - 2).toFixed(1),
        r: (s[1] * Math.min(1.6, strength)).toFixed(2)
      });
      c.animate(
        [
          { transform: 'translate(0,0)', opacity: 1, easing: 'cubic-bezier(0.2,0.7,0.4,1)' },
          { transform: 'translate(' + s[0] * strength + 'px,' + -16 * strength + 'px)', opacity: 1, offset: 0.5, easing: 'ease-in' },
          { transform: 'translate(' + s[0] * 1.5 * strength + 'px,2px)', opacity: 0 }
        ],
        { duration: 500, fill: 'both' }
      ).finished.then(
        function () {
          c.remove();
        },
        function () {}
      );
    });
    // wake the models the ripple passes, nearest first
    var reached = [];
    dots.forEach(function (d) {
      var dx = d.x - p.x,
        dy = (d.y - p.y) / K,
        dist = Math.sqrt(dx * dx + dy * dy);
      if (dist > R) return;
      var f = dist / R,
        delay = f * dur * 0.92;
      d.node.animate(
        [
          { opacity: 0.16, transform: 'scale(1)' },
          { opacity: 1 - 0.45 * f, transform: 'scale(' + (1.9 - 0.6 * f) + ')', offset: 0.12 },
          { opacity: 0.16, transform: 'scale(1)' }
        ],
        { duration: 900, delay: delay, easing: 'ease-out' }
      );
      if (d.depth > 0.45 && f > 0.12 && f < 0.75) reached.push({ d: d, delay: delay });
    });
    // and name a few of them, spaced apart, never the same name twice
    var want = Math.round(1 + strength * 1.5),
      picked = [];
    reached.sort(function () {
      return Math.random() - 0.5;
    });
    reached.forEach(function (h) {
      if (picked.length >= want) return;
      var clear = picked.every(function (q) {
        return Math.abs(q.d.x - h.d.x) > 90 || Math.abs(q.d.y - h.d.y) > 22;
      });
      if (clear) picked.push(h);
    });
    var names = MODELS.slice().sort(function () {
      return Math.random() - 0.5;
    });
    picked.forEach(function (h, i) {
      var t = el('text', { class: 'drip-name', x: h.d.x.toFixed(1), y: (h.d.y - 7).toFixed(1), 'text-anchor': 'middle' });
      t.textContent = names[i % names.length];
      t.animate(
        [
          { opacity: 0, transform: 'translateY(4px)' },
          { opacity: 1, transform: 'translateY(0)', offset: 0.18 },
          { opacity: 1, offset: 0.7 },
          { opacity: 0, transform: 'translateY(-3px)' }
        ],
        { duration: 1700, delay: h.delay + 60, fill: 'both', easing: 'ease-out' }
      ).finished.then(
        function () {
          t.remove();
        },
        function () {}
      );
    });
  }

  // fall from wherever the drop hangs to its landing point, flatten, splash
  function fall(drop, stem) {
    var s = drop.size,
      x = drop.p.x,
      landY = drop.p.y;
    var from = getComputedStyle(drop.g).transform;
    var endTop = landY - H * s * 1.16,
      dist = Math.max(40, endTop - TAP_Y),
      ms = 520 * Math.sqrt(dist / 180);
    drop.g
      .animate([{ transform: from }, { transform: tf(x, endTop, s * 0.9, s * 1.16) }], {
        duration: ms,
        easing: 'cubic-bezier(0.55, 0, 0.9, 0.5)',
        fill: 'forwards'
      })
      .finished.then(
        function () {
          splash(drop.p, Math.max(0.6, s));
          stem.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 500, fill: 'both' }).finished.then(
            function () {
              stem.remove();
            },
            function () {}
          );
          drop.g
            .animate(
              [
                { transform: tf(x, endTop, s * 0.9, s * 1.16), opacity: 1 },
                { transform: tf(x, landY - H * s * 0.25, s * 1.5, s * 0.25), opacity: 1, offset: 0.4 },
                { transform: tf(x, landY - H * s * 0.1, s * 1.7, s * 0.1), opacity: 0 }
              ],
              { duration: 150, fill: 'forwards' }
            )
            .finished.then(
              function () {
                drop.g.remove();
                inFlight--;
                settle();
              },
              function () {}
            );
        },
        function () {}
      );
  }

  // one pointer at a time: press starts a drop above the pressed point, release lets it go
  var held = null,
    raf = 0;
  function grow() {
    var t = performance.now() - held.t0;
    var size = t < HOLD_MS ? 0.2 + (0.8 * t) / HOLD_MS : Math.min(2.2, 1 + (t - HOLD_MS) / 1100);
    var neck = Math.min(1, t / 1600);
    var wobble = size > 1.6 ? Math.sin(performance.now() / 70) * 0.015 : 0;
    held.size = size + wobble;
    held.g.style.transform = tf(held.p.x, TAP_Y + neck * 2, held.size * (1 - 0.07 * neck), held.size * (1 + 0.13 * neck));
    raf = requestAnimationFrame(grow);
  }
  function onDown(e) {
    if (held || (e.button !== undefined && e.button > 0)) return;
    goLive();
    inFlight++;
    var p = toSurface(e);
    var stem = el('rect', { class: 'drip-stem', x: (p.x - 0.4).toFixed(1), y: -28, width: 0.8, height: 46, fill: 'url(#drip-stem)' });
    stem.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200, fill: 'both' });
    var g = el('g', { class: 'drip-live-drop' });
    var path = document.createElementNS(NS, 'path');
    path.setAttribute('class', 'drip-mark');
    path.setAttribute('d', markD);
    path.setAttribute('transform', 'scale(1.6)');
    g.appendChild(path);
    g.style.transformBox = 'fill-box';
    g.style.transformOrigin = '50% 0%';
    held = { g: g, p: p, stem: stem, t0: performance.now(), size: 0.2 };
    grow();
  }
  function onUp() {
    if (!held) return;
    cancelAnimationFrame(raf);
    var drop = held,
      quick = performance.now() - drop.t0 < HOLD_MS;
    held = null;
    if (!quick) {
      fall(drop, drop.stem);
      return;
    }
    // a tap: finish gathering a normal drop, then let it go
    var from = getComputedStyle(drop.g).transform;
    drop.size = 1;
    drop.g
      .animate([{ transform: from }, { transform: tf(drop.p.x, TAP_Y + 1, 0.95, 1.08) }], {
        duration: 320,
        easing: 'ease-out',
        fill: 'forwards'
      })
      .finished.then(
        function () {
          fall(drop, drop.stem);
        },
        function () {}
      );
  }

  // release is watched on the window so a drop let go outside the picture still falls
  svg.addEventListener('pointerdown', onDown);
  window.addEventListener('pointerup', onUp);
  window.addEventListener('pointercancel', onUp);
  return function () {
    cancelAnimationFrame(raf);
    clearTimeout(idleTimer);
    svg.removeEventListener('pointerdown', onDown);
    window.removeEventListener('pointerup', onUp);
    window.removeEventListener('pointercancel', onUp);
    layer.remove();
    root.classList.remove('drip-live');
  };
}
