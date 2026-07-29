;(function () {
  var canvas = document.getElementById("hero-rain")
  if (!canvas) return
  var ctx = canvas.getContext("2d")
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
  var JIT = new Float32Array(64)
  for (var q = 0; q < 64; q++) {
    JIT[q] = 0.62 + Math.random() * 0.38
  }

  function readTheme() {
    var cs = getComputedStyle(document.documentElement)
    var ink = trip(cs.getPropertyValue("--text"), "23,23,23")
    paper = trip(cs.getPropertyValue("--bg"), "250,250,250")
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
    d.speed = ((8 + Math.random() * 26) * dpr) / c.size
    d.alpha = 0.24 + Math.random() * 0.44
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
        gap: Math.max(1, Math.round(size * 0.2)),
        rows: Math.ceil(H / size)
      })
      x += size
    }
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
        if (k === 0) a = Math.min(0.94, d.alpha * 2.0)
        else if (k === 1) a = Math.min(0.72, d.alpha * 1.35)
        var s = (a * 48) | 0
        if (s < 1) continue
        var side2 = k < 2 ? side : Math.max(1, Math.round(side * (0.32 + 0.68 * t)))
        ctx.fillStyle = shades[s]
        ctx.fillRect(c.x + ((side - side2) >> 1), y, side2, side2)
      }
    }
  }

  var running = false
  var inView = true
  var frameInterval = 1000 / 30
  var frameTimer
  function loop() {
    if (!running) return
    var now = performance.now()
    step(Math.min(0.05, (now - last) / 1000))
    last = now
    draw()
    frameTimer = window.setTimeout(loop, frameInterval)
  }
  function play() {
    if (running || reduce.matches || !inView || document.hidden) return
    running = true
    last = performance.now()
    frameTimer = window.setTimeout(loop, frameInterval)
  }
  function pause() {
    running = false
    window.clearTimeout(frameTimer)
    last = 0
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
  if ("IntersectionObserver" in window) {
    inView = false
    new IntersectionObserver(function (e) {
      inView = e[0].isIntersecting
      inView ? play() : pause()
    }).observe(canvas)
  } else if (!reduce.matches) {
    play()
  }
  document.addEventListener("visibilitychange", function () {
    document.hidden ? pause() : play()
  })
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
