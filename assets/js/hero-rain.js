/* Landing splash — matrix rain in the site's monochrome palette. Fine columns
   of square pixels fall from the ceiling, each column packing several drops at
   once with its own block size, speed, trail length and weight, so the field
   never empties out. Positions are carried as floats and rounded to device
   pixels at draw time, so a trail slides instead of stepping row to row.
   Canvas2D, no deps. Re-themes light/dark, holds a static frame under
   prefers-reduced-motion, and pauses when scrolled off-screen. */
;(function () {
  var canvas = document.getElementById("hero-rain")
  if (!canvas) return
  /* CPU-backed (willReadFrequently) so the canvas isn't promoted to its own GPU
     compositing layer — otherwise the bio text stacked above it loses subpixel
     antialiasing and renders soft/grayscale next to the rest of the page */
  var ctx = canvas.getContext("2d", { willReadFrequently: true })
  if (!ctx) return

  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)")
  var dpr = 1,
    W = 0,
    H = 0,
    columns = [],
    drops = [],
    last = 0
  var paper = "250,250,250",
    shades = []
  /* per-cell brightness jitter, keyed on column+row so it sits still in space
     while a trail falls through it — without it neighbouring blocks land on
     near-identical alphas and the trail smears into one gradient bar */
  var JIT = new Float32Array(64)
  for (var q = 0; q < 64; q++) {
    JIT[q] = 0.62 + Math.random() * 0.38
  }

  function readTheme() {
    var cs = getComputedStyle(document.documentElement)
    var ink = trip(cs.getPropertyValue("--text"), "23,23,23")
    paper = trip(cs.getPropertyValue("--bg"), "250,250,250")
    // prebuilt alpha ramp: a few thousand rects a frame, none of them paying
    // for string building
    shades = []
    for (var i = 0; i <= 48; i++) {
      shades.push("rgba(" + ink + "," + (i / 48).toFixed(3) + ")")
    }
  }
  function trip(h, fallback) {
    h = (h || "").trim().replace(/^#/, "")
    if (h.length === 3) h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2]
    if (h.length < 6) return fallback
    var n = parseInt(h, 16)
    return ((n >> 16) & 255) + "," + ((n >> 8) & 255) + "," + (n & 255)
  }

  function reset(d, seeded) {
    var c = columns[d.c]
    d.len = 8 + Math.floor(Math.random() * c.rows * 0.42)
    // pick a fall rate in css px/s, then convert to this column's rows/s so
    // small blocks don't crawl next to big ones
    d.speed = ((16 + Math.random() * 52) * dpr) / c.size
    d.alpha = 0.24 + Math.random() * 0.44
    // short re-entry gap only — a long one leaves visible holes in the field
    d.head = seeded
      ? Math.random() * (c.rows + d.len)
      : -d.len - Math.random() * c.rows * 0.08
  }

  function seed() {
    columns = []
    drops = []
    var unit = Math.max(3, Math.round(3 * dpr))
    var sizes = [unit, unit * 1.35, unit * 1.8]
    var x = 0
    while (x < W) {
      var size = Math.max(3, Math.round(sizes[(Math.random() * 3) | 0]))
      if (x + size > W) size = W - x
      columns.push({
        x: x,
        size: size,
        // visible gap between cells, so a trail reads as stacked pixels
        gap: Math.max(1, Math.round(size * 0.2)),
        rows: Math.ceil(H / size)
      })
      x += size
    }
    // every column carries two independent meteors, and each re-enters quickly
    // after burning out, so the field's weight stays constant
    for (var i = 0; i < columns.length; i++) {
      for (var k = 0; k < 2; k++) {
        var d = { c: i }
        reset(d, true)
        drops.push(d)
      }
    }
  }

  function step(dt) {
    for (var i = 0; i < drops.length; i++) {
      var d = drops[i]
      d.head += d.speed * dt
      if (d.head - d.len > columns[d.c].rows) reset(d, false)
    }
  }

  function draw() {
    ctx.fillStyle = "rgb(" + paper + ")"
    ctx.fillRect(0, 0, W, H)
    for (var i = 0; i < drops.length; i++) {
      var d = drops[i]
      var c = columns[d.c]
      var base = d.head * c.size
      var cell = Math.floor(d.head)
      var side = c.size - c.gap
      for (var k = 0; k < d.len; k++) {
        var y = Math.round(base - k * c.size)
        if (y + side < 0) continue
        if (y > H) continue
        var t = 1 - k / d.len
        var a = d.alpha * t * (0.34 + 0.66 * t) * JIT[(d.c * 31 + (cell - k) * 17) & 63]
        // the leading pair is the meteor's core, at full width and near-solid
        if (k === 0) a = Math.min(0.94, d.alpha * 2.0)
        else if (k === 1) a = Math.min(0.72, d.alpha * 1.35)
        var s = (a * 48) | 0
        if (s < 1) continue
        // tail narrows as it burns out, so it dissolves into loose pixels
        var side2 = k < 2 ? side : Math.max(1, Math.round(side * (0.32 + 0.68 * t)))
        ctx.fillStyle = shades[s]
        ctx.fillRect(c.x + ((side - side2) >> 1), y, side2, side2)
      }
    }
  }

  var running = false
  function loop(now) {
    if (!running) return
    if (last) step(Math.min(0.05, (now - last) / 1000))
    last = now
    draw()
    requestAnimationFrame(loop)
  }
  function play() {
    if (running || reduce.matches) return
    running = true
    last = 0
    requestAnimationFrame(loop)
  }
  function pause() {
    running = false
  }

  function resize() {
    dpr = Math.min(window.devicePixelRatio || 1, 2)
    var w = Math.max(1, Math.round(canvas.clientWidth * dpr))
    var h = Math.max(1, Math.round(canvas.clientHeight * dpr))
    if (w === W && h === H) return
    W = canvas.width = w
    H = canvas.height = h
    readTheme()
    seed()
    draw()
  }

  // debounce resize: a window drag fires dozens of events/sec and each one
  // re-seeds every column — coalesce them so that happens once the drag settles
  var resizeTimer
  window.addEventListener("resize", function () {
    window.clearTimeout(resizeTimer)
    resizeTimer = window.setTimeout(resize, 150)
  })
  new MutationObserver(function () {
    readTheme()
    draw()
  }).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"]
  })

  resize()
  if (reduce.matches) {
    draw()
  } else if ("IntersectionObserver" in window) {
    new IntersectionObserver(function (e) {
      e[0].isIntersecting ? play() : pause()
    }).observe(canvas)
  } else {
    play()
  }
  if (reduce.addEventListener) {
    reduce.addEventListener("change", function () {
      if (reduce.matches) {
        pause()
        draw()
      } else {
        play()
      }
    })
  }
})()
