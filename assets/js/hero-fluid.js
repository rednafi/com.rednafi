/* Landing splash — a particle-based fluid field (à la vercel.com/fluid), but in
   the site's monochrome palette. Hundreds of ink particles are advected by a
   curl-noise velocity field (divergence-free → swirling, liquid-like motion)
   and smeared into flowing streaks by a fading paper trail. Density falls off
   toward the bottom-left so the bio + icons stay clean. Canvas2D, no deps.
   Re-themes light/dark, settles to a static frame under prefers-reduced-motion,
   and pauses when scrolled off-screen. */
;(function () {
  var canvas = document.getElementById("hero-fluid")
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
    parts = [],
    N = 0,
    t = 0
  var ink = "23,23,23",
    paper = "250,250,250"

  function readTheme() {
    var cs = getComputedStyle(document.documentElement)
    ink = hexToTriplet(cs.getPropertyValue("--text"), "23,23,23")
    paper = hexToTriplet(cs.getPropertyValue("--bg"), "250,250,250")
  }
  function hexToTriplet(h, fallback) {
    h = (h || "").trim().replace(/^#/, "")
    if (h.length === 3) h = h[0] + h[0] + h[1] + h[1] + h[2] + h[2]
    if (h.length < 6) return fallback
    var n = parseInt(h, 16)
    return ((n >> 16) & 255) + "," + ((n >> 8) & 255) + "," + (n & 255)
  }

  // cheap value noise + 3-octave fbm, curl gives a divergence-free flow field
  function hash(x, y) {
    var n = Math.sin(x * 127.1 + y * 311.7) * 43758.5453
    return n - Math.floor(n)
  }
  function vnoise(x, y) {
    var xi = Math.floor(x),
      yi = Math.floor(y),
      xf = x - xi,
      yf = y - yi
    var u = xf * xf * (3 - 2 * xf),
      v = yf * yf * (3 - 2 * yf)
    var a = hash(xi, yi),
      b = hash(xi + 1, yi),
      c = hash(xi, yi + 1),
      d = hash(xi + 1, yi + 1)
    return a + (b - a) * u + (c - a) * v + (a - b - c + d) * u * v
  }
  function fbm(x, y) {
    var s = 0,
      amp = 0.5,
      f = 1
    for (var i = 0; i < 3; i++) {
      s += amp * vnoise(x * f, y * f)
      f *= 2
      amp *= 0.5
    }
    return s
  }
  function curl(x, y) {
    var e = 0.6
    var dydp = (fbm(x, y + e) - fbm(x, y - e)) / (2 * e)
    var dxdp = (fbm(x + e, y) - fbm(x - e, y)) / (2 * e)
    return [dydp, -dxdp]
  }

  function ss(a, b, x) {
    x = (x - a) / (b - a)
    x = x < 0 ? 0 : x > 1 ? 1 : x
    return x * x * (3 - 2 * x)
  }
  // a low horizontal band: rises into view through the lower portion (soft at
  // the top edge), faded at the left/right sides — identical at any width, and
  // it sits below the centred copy.
  function density(px, py) {
    var nx = px / W,
      ny = py / H
    var band = ss(0.52, 0.96, ny)
    var sides = ss(0, 0.13, nx) * ss(0, 0.13, 1 - nx)
    return band * sides
  }

  function spawn(p) {
    p[0] = Math.random() * W
    p[1] = Math.random() * H
    p[2] = 40 + Math.random() * 80 // life in frames
  }
  function seed() {
    parts = []
    N = Math.max(260, Math.min(1700, Math.round((W * H) / (1700 * dpr))))
    for (var i = 0; i < N; i++) {
      var p = [0, 0, 0]
      spawn(p)
      parts.push(p)
    }
  }

  function step() {
    // fade the previous frame toward paper → motion-blur streaks
    ctx.fillStyle = "rgba(" + paper + ",0.085)"
    ctx.fillRect(0, 0, W, H)

    var scale = 0.0042 // noise frequency in px space
    var speed = 1.625 * dpr
    var r = Math.max(1, 1.15 * dpr)
    for (var i = 0; i < N; i++) {
      var p = parts[i]
      var c = curl(p[0] * scale + t, p[1] * scale - t)
      p[0] += c[0] * speed
      p[1] += c[1] * speed
      p[2] -= 1
      if (p[2] <= 0 || p[0] < -4 || p[0] > W + 4 || p[1] < -4 || p[1] > H + 4) {
        spawn(p)
        continue
      }
      var a = density(p[0], p[1]) * 0.5
      if (a <= 0.01) continue
      ctx.fillStyle = "rgba(" + ink + "," + a.toFixed(3) + ")"
      ctx.beginPath()
      ctx.arc(p[0], p[1], r, 0, 6.2832)
      ctx.fill()
    }
    t += 0.0001
  }

  function clear() {
    ctx.fillStyle = "rgb(" + paper + ")"
    ctx.fillRect(0, 0, W, H)
  }

  var running = false
  function loop() {
    if (!running) return
    step()
    requestAnimationFrame(loop)
  }
  function play() {
    if (running || reduce.matches) return
    running = true
    requestAnimationFrame(loop)
  }
  function pause() {
    running = false
  }
  function still() {
    // accumulate trails into one settled frame, then hold
    clear()
    for (var i = 0; i < 220; i++) step()
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
    clear()
    if (reduce.matches) still()
  }

  // debounce resize: a window drag fires dozens of events/sec, and each resize
  // re-seeds + (under reduced-motion) runs a synchronous settle pass — coalesce
  // them so that work happens once the drag settles, not every pixel
  var resizeTimer
  window.addEventListener("resize", function () {
    window.clearTimeout(resizeTimer)
    resizeTimer = window.setTimeout(resize, 150)
  })
  new MutationObserver(function () {
    readTheme()
    clear()
    if (reduce.matches) still()
  }).observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"]
  })

  resize()
  if (reduce.matches) {
    still()
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
        still()
      } else {
        play()
      }
    })
  }
})()
