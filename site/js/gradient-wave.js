// ============================================================
// MalleusOS — animowane tło "gradient wave" (czysty WebGL)
//
// Port komponentu React (shadcn "GradientWave") na waniliowy JS,
// bo /site z zasady nie używa frameworków. WebGL to API rysowania
// kartą graficzną: wierzchołki płaszczyzny faluje szum simplex
// w shaderze (programie działającym na GPU), a kolory warstw
// mieszają się w gradient.
//
// Ograniczenie shadera: maksymalnie 4 kolory (wektor vec4
// u_active_colors) — pierwszy to baza, kolejne to warstwy fal.
// ============================================================
(function () {
  "use strict";

  // Zamiana koloru hex (np. 0x53d6e8) na trzy składowe RGB 0–1,
  // bo tak przyjmuje je shader.
  function normalizeColor(hexCode) {
    return [
      ((hexCode >> 16) & 255) / 255,
      ((hexCode >> 8) & 255) / 255,
      (255 & hexCode) / 255,
    ];
  }

  // ---------- MiniGl: malutka nakładka na surowe WebGL ----------
  class MiniGl {
    constructor(canvas) {
      this.canvas = canvas;
      this.meshes = [];
      const gl = canvas.getContext("webgl", { antialias: true });
      if (!gl) throw new Error("Brak wsparcia WebGL");
      this.gl = gl;

      const context = gl;
      const _miniGl = this;

      // Uniform = wartość przekazywana z JS do shadera (kolor, czas…)
      this.Uniform = class {
        constructor(e) {
          this.type = "float";
          Object.assign(this, e);
          const typeMap = {
            float: "1f",
            int: "1i",
            vec2: "2fv",
            vec3: "3fv",
            vec4: "4fv",
            mat4: "Matrix4fv",
          };
          this.typeFn = typeMap[this.type] || "1f";
        }

        update(location) {
          if (this.value === undefined || location === null) return;
          const isMatrix = this.typeFn.indexOf("Matrix") === 0;
          const fn = "uniform" + this.typeFn;
          if (isMatrix) {
            context[fn](location, this.transpose || false, this.value);
          } else {
            context[fn](location, this.value);
          }
        }

        // Generuje deklarację uniformu w kodzie GLSL shadera.
        getDeclaration(name, type, length) {
          if (this.excludeFrom === type) return "";

          if (this.type === "array") {
            return (
              this.value[0].getDeclaration(name, type, this.value.length) +
              "\nconst int " + name + "_length = " + this.value.length + ";"
            );
          }

          if (this.type === "struct") {
            let nameNoPrefix = name.replace("u_", "");
            nameNoPrefix =
              nameNoPrefix.charAt(0).toUpperCase() + nameNoPrefix.slice(1);
            const fields = Object.entries(this.value)
              .map(([n, u]) => u.getDeclaration(n, type).replace(/^uniform/, ""))
              .join("");
            return (
              "uniform struct " + nameNoPrefix + " \n{\n" + fields + "\n} " +
              name + (length ? "[" + length + "]" : "") + ";"
            );
          }

          return (
            "uniform " + this.type + " " + name +
            (length ? "[" + length + "]" : "") + ";"
          );
        }
      };

      // Attribute = dane per-wierzchołek (pozycje, współrzędne UV)
      this.Attribute = class {
        constructor(e) {
          this.type = context.FLOAT;
          this.normalized = false;
          this.buffer = context.createBuffer();
          Object.assign(this, e);
        }

        update() {
          if (this.values) {
            context.bindBuffer(this.target, this.buffer);
            context.bufferData(this.target, this.values, context.STATIC_DRAW);
          }
        }

        attach(e, t) {
          const n = context.getAttribLocation(t, e);
          if (this.target === context.ARRAY_BUFFER) {
            context.bindBuffer(this.target, this.buffer);
            context.enableVertexAttribArray(n);
            context.vertexAttribPointer(n, this.size, this.type, this.normalized, 0, 0);
          }
          return n;
        }

        use(e) {
          context.bindBuffer(this.target, this.buffer);
          if (this.target === context.ARRAY_BUFFER) {
            context.enableVertexAttribArray(e);
            context.vertexAttribPointer(e, this.size, this.type, this.normalized, 0, 0);
          }
        }
      };

      // Material = skompilowana para shaderów (vertex + fragment)
      this.Material = class {
        constructor(vertexShaders, fragments, uniforms) {
          const material = this;
          this.uniforms = uniforms || {};
          this.uniformInstances = [];

          function getShader(type, source) {
            const shader = context.createShader(type);
            context.shaderSource(shader, source);
            context.compileShader(shader);
            if (!context.getShaderParameter(shader, context.COMPILE_STATUS)) {
              console.error(context.getShaderInfoLog(shader));
              throw new Error("Błąd kompilacji shadera");
            }
            return shader;
          }

          function getUniformDeclarations(uniforms, type) {
            return Object.entries(uniforms)
              .map(([uniform, value]) => value.getDeclaration(uniform, type))
              .join("\n");
          }

          const prefix = "precision highp float;";

          const vertexSource =
            prefix +
            "\nattribute vec4 position;\nattribute vec2 uv;\nattribute vec2 uvNorm;\n" +
            getUniformDeclarations(_miniGl.commonUniforms, "vertex") + "\n" +
            getUniformDeclarations(material.uniforms, "vertex") + "\n" +
            vertexShaders;

          const fragmentSource =
            prefix + "\n" +
            getUniformDeclarations(_miniGl.commonUniforms, "fragment") + "\n" +
            getUniformDeclarations(material.uniforms, "fragment") + "\n" +
            fragments;

          material.program = context.createProgram();
          context.attachShader(material.program, getShader(context.VERTEX_SHADER, vertexSource));
          context.attachShader(material.program, getShader(context.FRAGMENT_SHADER, fragmentSource));
          context.linkProgram(material.program);

          if (!context.getProgramParameter(material.program, context.LINK_STATUS)) {
            console.error(context.getProgramInfoLog(material.program));
            throw new Error("Błąd linkowania programu");
          }

          context.useProgram(material.program);
          material.attachUniforms(undefined, _miniGl.commonUniforms);
          material.attachUniforms(undefined, material.uniforms);
        }

        attachUniforms(name, uniforms) {
          if (name === undefined) {
            Object.entries(uniforms).forEach(([n, u]) => this.attachUniforms(n, u));
          } else if (uniforms.type === "array") {
            uniforms.value.forEach((u, i) =>
              this.attachUniforms(name + "[" + i + "]", u)
            );
          } else if (uniforms.type === "struct") {
            Object.entries(uniforms.value).forEach(([u, i]) =>
              this.attachUniforms(name + "." + u, i)
            );
          } else {
            this.uniformInstances.push({
              uniform: uniforms,
              location: context.getUniformLocation(this.program, name),
            });
          }
        }
      };

      // PlaneGeometry = siatka trójkątów pokrywająca ekran
      this.PlaneGeometry = class {
        constructor() {
          this.width = 1;
          this.height = 1;
          this.vertexCount = 0;
          this.xSegCount = 0;
          this.ySegCount = 0;
          this.attributes = {
            position: new _miniGl.Attribute({ target: context.ARRAY_BUFFER, size: 3 }),
            uv: new _miniGl.Attribute({ target: context.ARRAY_BUFFER, size: 2 }),
            uvNorm: new _miniGl.Attribute({ target: context.ARRAY_BUFFER, size: 2 }),
            index: new _miniGl.Attribute({
              target: context.ELEMENT_ARRAY_BUFFER,
              size: 3,
              type: context.UNSIGNED_SHORT,
            }),
          };
        }

        setTopology(xSegs, ySegs) {
          this.xSegCount = xSegs || 1;
          this.ySegCount = ySegs || 1;
          this.vertexCount = (this.xSegCount + 1) * (this.ySegCount + 1);
          const quadCount = this.xSegCount * this.ySegCount * 2;

          this.attributes.uv.values = new Float32Array(2 * this.vertexCount);
          this.attributes.uvNorm.values = new Float32Array(2 * this.vertexCount);
          this.attributes.index.values = new Uint16Array(3 * quadCount);

          for (let y = 0; y <= this.ySegCount; y++) {
            for (let x = 0; x <= this.xSegCount; x++) {
              const i = y * (this.xSegCount + 1) + x;
              this.attributes.uv.values[2 * i] = x / this.xSegCount;
              this.attributes.uv.values[2 * i + 1] = 1 - y / this.ySegCount;
              this.attributes.uvNorm.values[2 * i] = (x / this.xSegCount) * 2 - 1;
              this.attributes.uvNorm.values[2 * i + 1] = 1 - (y / this.ySegCount) * 2;

              if (x < this.xSegCount && y < this.ySegCount) {
                const s = y * this.xSegCount + x;
                this.attributes.index.values[6 * s] = i;
                this.attributes.index.values[6 * s + 1] = i + 1 + this.xSegCount;
                this.attributes.index.values[6 * s + 2] = i + 1;
                this.attributes.index.values[6 * s + 3] = i + 1;
                this.attributes.index.values[6 * s + 4] = i + 1 + this.xSegCount;
                this.attributes.index.values[6 * s + 5] = i + 2 + this.xSegCount;
              }
            }
          }

          this.attributes.uv.update();
          this.attributes.uvNorm.update();
          this.attributes.index.update();
        }

        setSize(width, height) {
          this.width = width || 1;
          this.height = height || 1;
          this.attributes.position.values = new Float32Array(3 * this.vertexCount);

          const offsetX = this.width / -2;
          const offsetY = this.height / -2;
          const segWidth = this.width / this.xSegCount;
          const segHeight = this.height / this.ySegCount;

          for (let y = 0; y <= this.ySegCount; y++) {
            const posY = offsetY + y * segHeight;
            for (let x = 0; x <= this.xSegCount; x++) {
              const posX = offsetX + x * segWidth;
              const idx = y * (this.xSegCount + 1) + x;
              this.attributes.position.values[3 * idx] = posX;
              this.attributes.position.values[3 * idx + 1] = -posY;
              this.attributes.position.values[3 * idx + 2] = 0;
            }
          }

          this.attributes.position.update();
        }
      };

      // Mesh = geometria + materiał, czyli gotowy obiekt do narysowania
      this.Mesh = class {
        constructor(geometry, material) {
          this.geometry = geometry;
          this.material = material;
          this.attributeInstances = [];

          Object.entries(this.geometry.attributes).forEach(([e, attribute]) => {
            this.attributeInstances.push({
              attribute: attribute,
              location: attribute.attach(e, this.material.program),
            });
          });

          _miniGl.meshes.push(this);
        }

        draw() {
          context.useProgram(this.material.program);
          this.material.uniformInstances.forEach(({ uniform, location }) =>
            uniform.update(location)
          );
          this.attributeInstances.forEach(({ attribute, location }) =>
            attribute.use(location)
          );
          context.drawElements(
            context.TRIANGLES,
            this.geometry.attributes.index.values.length,
            context.UNSIGNED_SHORT,
            0
          );
        }
      };

      const identityMatrix = [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1];
      this.commonUniforms = {
        projectionMatrix: new this.Uniform({ type: "mat4", value: identityMatrix }),
        modelViewMatrix: new this.Uniform({ type: "mat4", value: identityMatrix }),
        resolution: new this.Uniform({ type: "vec2", value: [1, 1] }),
        aspectRatio: new this.Uniform({ type: "float", value: 1 }),
      };
    }

    setSize(w, h) {
      this.width = w;
      this.height = h;
      this.canvas.width = w;
      this.canvas.height = h;
      this.gl.viewport(0, 0, w, h);
      this.commonUniforms.resolution.value = [w, h];
      this.commonUniforms.aspectRatio.value = w / h;
    }

    setOrthographicCamera() {
      this.commonUniforms.projectionMatrix.value = [
        2 / this.width, 0, 0, 0,
        0, 2 / this.height, 0, 0,
        0, 0, -0.001, 0,
        0, 0, 0, 1,
      ];
    }

    render() {
      this.gl.clearColor(0, 0, 0, 0);
      this.gl.clearDepth(1);
      this.meshes.forEach((m) => m.draw());
    }
  }

  // ---------- Gradient: fale kolorów na płaszczyźnie ----------
  class Gradient {
    constructor(canvas, colors) {
      this.canvas = canvas;
      this.colors = colors;
      this.time = 0;
      this.last = 0;
      this.isPlaying = false;
      this.minigl = new MiniGl(canvas);
      this.init();
    }

    init() {
      const minigl = this.minigl;
      const sectionColors = this.colors.map((hex) =>
        normalizeColor(parseInt(hex.replace("#", "0x"), 16))
      );

      const uniforms = {
        u_time: new minigl.Uniform({ value: 0 }),
        u_shadow_power: new minigl.Uniform({ value: 5 }),
        u_darken_top: new minigl.Uniform({ value: 0 }),
        u_active_colors: new minigl.Uniform({ value: [1, 1, 1, 1], type: "vec4" }),
        u_global: new minigl.Uniform({
          value: {
            noiseFreq: new minigl.Uniform({ value: [0.00014, 0.00029], type: "vec2" }),
            noiseSpeed: new minigl.Uniform({ value: 0.000005 }),
          },
          type: "struct",
        }),
        u_vertDeform: new minigl.Uniform({
          value: {
            incline: new minigl.Uniform({ value: 0 }),
            offsetTop: new minigl.Uniform({ value: -0.5 }),
            offsetBottom: new minigl.Uniform({ value: -0.5 }),
            noiseFreq: new minigl.Uniform({ value: [3, 4], type: "vec2" }),
            noiseAmp: new minigl.Uniform({ value: 320 }),
            noiseSpeed: new minigl.Uniform({ value: 10 }),
            noiseFlow: new minigl.Uniform({ value: 3 }),
            noiseSeed: new minigl.Uniform({ value: 5 }),
          },
          type: "struct",
          excludeFrom: "fragment",
        }),
        u_baseColor: new minigl.Uniform({
          value: sectionColors[0],
          type: "vec3",
          excludeFrom: "fragment",
        }),
        u_waveLayers: new minigl.Uniform({ value: [], excludeFrom: "fragment", type: "array" }),
      };

      for (let i = 1; i < sectionColors.length; i++) {
        uniforms.u_waveLayers.value.push(
          new minigl.Uniform({
            value: {
              color: new minigl.Uniform({ value: sectionColors[i], type: "vec3" }),
              noiseFreq: new minigl.Uniform({
                value: [2 + i / sectionColors.length, 3 + i / sectionColors.length],
                type: "vec2",
              }),
              noiseSpeed: new minigl.Uniform({ value: 11 + 0.3 * i }),
              noiseFlow: new minigl.Uniform({ value: 6.5 + 0.3 * i }),
              noiseSeed: new minigl.Uniform({ value: 5 + 10 * i }),
              noiseFloor: new minigl.Uniform({ value: 0.1 }),
              noiseCeil: new minigl.Uniform({ value: 0.63 + 0.07 * i }),
            },
            type: "struct",
          })
        );
      }

      // Vertex shader: szum simplex 3D (klasyczna implementacja
      // Ashimy Arts) faluje wierzchołki i miesza kolory warstw.
      const vertexShader = [
        "vec3 mod289(vec3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }",
        "vec4 mod289(vec4 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }",
        "vec4 permute(vec4 x) { return mod289(((x*34.0)+1.0)*x); }",
        "vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }",
        "",
        "float snoise(vec3 v) {",
        "  const vec2 C = vec2(1.0/6.0, 1.0/3.0);",
        "  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);",
        "  vec3 i  = floor(v + dot(v, C.yyy));",
        "  vec3 x0 = v - i + dot(i, C.xxx);",
        "  vec3 g = step(x0.yzx, x0.xyz);",
        "  vec3 l = 1.0 - g;",
        "  vec3 i1 = min(g.xyz, l.zxy);",
        "  vec3 i2 = max(g.xyz, l.zxy);",
        "  vec3 x1 = x0 - i1 + C.xxx;",
        "  vec3 x2 = x0 - i2 + C.yyy;",
        "  vec3 x3 = x0 - D.yyy;",
        "  i = mod289(i);",
        "  vec4 p = permute(permute(permute(i.z + vec4(0.0, i1.z, i2.z, 1.0)) + i.y + vec4(0.0, i1.y, i2.y, 1.0)) + i.x + vec4(0.0, i1.x, i2.x, 1.0));",
        "  float n_ = 0.142857142857;",
        "  vec3 ns = n_ * D.wyz - D.xzx;",
        "  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);",
        "  vec4 x_ = floor(j * ns.z);",
        "  vec4 y_ = floor(j - 7.0 * x_);",
        "  vec4 x = x_ *ns.x + ns.yyyy;",
        "  vec4 y = y_ *ns.x + ns.yyyy;",
        "  vec4 h = 1.0 - abs(x) - abs(y);",
        "  vec4 b0 = vec4(x.xy, y.xy);",
        "  vec4 b1 = vec4(x.zw, y.zw);",
        "  vec4 s0 = floor(b0)*2.0 + 1.0;",
        "  vec4 s1 = floor(b1)*2.0 + 1.0;",
        "  vec4 sh = -step(h, vec4(0.0));",
        "  vec4 a0 = b0.xzyw + s0.xzyw*sh.xxyy;",
        "  vec4 a1 = b1.xzyw + s1.xzyw*sh.zzww;",
        "  vec3 p0 = vec3(a0.xy,h.x);",
        "  vec3 p1 = vec3(a0.zw,h.y);",
        "  vec3 p2 = vec3(a1.xy,h.z);",
        "  vec3 p3 = vec3(a1.zw,h.w);",
        "  vec4 norm = taylorInvSqrt(vec4(dot(p0,p0), dot(p1,p1), dot(p2, p2), dot(p3,p3)));",
        "  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;",
        "  vec4 m = max(0.6 - vec4(dot(x0,x0), dot(x1,x1), dot(x2,x2), dot(x3,x3)), 0.0);",
        "  m = m * m;",
        "  return 42.0 * dot(m*m, vec4(dot(p0,x0), dot(p1,x1), dot(p2,x2), dot(p3,x3)));",
        "}",
        "",
        "vec3 blendNormal(vec3 base, vec3 blend) { return blend; }",
        "vec3 blendNormal(vec3 base, vec3 blend, float opacity) { return (blend * opacity + base * (1.0 - opacity)); }",
        "",
        "varying vec3 v_color;",
        "",
        "void main() {",
        "  float time = u_time * u_global.noiseSpeed;",
        "  vec2 noiseCoord = resolution * uvNorm * u_global.noiseFreq;",
        "  float tilt = resolution.y / 2.0 * uvNorm.y;",
        "  float incline = resolution.x * uvNorm.x / 2.0 * u_vertDeform.incline;",
        "  float offset = resolution.x / 2.0 * u_vertDeform.incline * mix(u_vertDeform.offsetBottom, u_vertDeform.offsetTop, uv.y);",
        "",
        "  float noise = snoise(vec3(",
        "    noiseCoord.x * u_vertDeform.noiseFreq.x + time * u_vertDeform.noiseFlow,",
        "    noiseCoord.y * u_vertDeform.noiseFreq.y,",
        "    time * u_vertDeform.noiseSpeed + u_vertDeform.noiseSeed",
        "  )) * u_vertDeform.noiseAmp;",
        "",
        "  noise *= 1.0 - pow(abs(uvNorm.y), 2.0);",
        "  noise = max(0.0, noise);",
        "",
        "  vec3 pos = vec3(position.x, position.y + tilt + incline + noise - offset, position.z);",
        "",
        "  v_color = u_baseColor;",
        "",
        "  for (int i = 0; i < u_waveLayers_length; i++) {",
        "    if (u_active_colors[i + 1] == 1.) {",
        "      WaveLayers layer = u_waveLayers[i];",
        "      float layerNoise = smoothstep(",
        "        layer.noiseFloor,",
        "        layer.noiseCeil,",
        "        snoise(vec3(",
        "          noiseCoord.x * layer.noiseFreq.x + time * layer.noiseFlow,",
        "          noiseCoord.y * layer.noiseFreq.y,",
        "          time * layer.noiseSpeed + layer.noiseSeed",
        "        )) / 2.0 + 0.5",
        "      );",
        "      v_color = blendNormal(v_color, layer.color, pow(layerNoise, 4.));",
        "    }",
        "  }",
        "",
        "  gl_Position = projectionMatrix * modelViewMatrix * vec4(pos, 1.0);",
        "}",
      ].join("\n");

      const fragmentShader = [
        "varying vec3 v_color;",
        "",
        "void main() {",
        "  vec3 color = v_color;",
        "  if (u_darken_top == 1.0) {",
        "    vec2 st = gl_FragCoord.xy/resolution.xy;",
        "    color.g -= pow(st.y + sin(-12.0) * st.x, u_shadow_power) * 0.4;",
        "  }",
        "  gl_FragColor = vec4(color, 1.0);",
        "}",
      ].join("\n");

      const material = new minigl.Material(vertexShader, fragmentShader, uniforms);
      const geometry = new minigl.PlaneGeometry();
      this.mesh = new minigl.Mesh(geometry, material);

      this.resize();
      window.addEventListener("resize", () => this.resize());
    }

    resize() {
      const width = window.innerWidth;
      const height = window.innerHeight;
      this.minigl.setSize(width, height);
      this.minigl.setOrthographicCamera();

      const xSegCount = Math.ceil(width * 0.02);
      const ySegCount = Math.ceil(height * 0.05);
      this.mesh.geometry.setTopology(xSegCount, ySegCount);
      this.mesh.geometry.setSize(width, height);
    }

    animate = (timestamp) => {
      if (!this.isPlaying) return;
      // Ograniczamy skok czasu (np. po powrocie do karty),
      // żeby animacja nie "teleportowała się".
      this.time += Math.min(timestamp - this.last, 1000 / 15);
      this.last = timestamp;
      this.mesh.material.uniforms.u_time.value = this.time;
      this.minigl.render();
      this.animationId = requestAnimationFrame(this.animate);
    };

    start() {
      this.isPlaying = true;
      this.animationId = requestAnimationFrame(this.animate);
    }

    stop() {
      this.isPlaying = false;
      if (this.animationId) cancelAnimationFrame(this.animationId);
    }
  }

  // ---------- Start: podpinamy tło pod stronę ----------
  // Paleta MalleusOS: baza = tło strony, potem coraz jaśniejsze
  // granaty aż po stłumiony cyjan. Maks. 4 kolory (patrz wyżej).
  const KOLORY = ["#0d0b1a", "#1a1440", "#2b1e5e", "#45318a"];

  const canvas = document.getElementById("bg-wave");
  if (!canvas) return;

  try {
    const gradient = new Gradient(canvas, KOLORY);

    // Ustawienia przeniesione z propsów komponentu React:
    // wolniejszy, spokojniejszy ruch niż w oryginale.
    const u = gradient.mesh.material.uniforms;
    u.u_global.value.noiseFreq.value = [0.0001, 0.0009];
    u.u_global.value.noiseSpeed.value = 0.00001;
    u.u_vertDeform.value.noiseAmp.value = 250;
    u.u_vertDeform.value.noiseFlow.value = 5;

    // Dostępność: przy "ogranicz animacje" rysujemy jedną
    // nieruchomą klatkę zamiast animować.
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      gradient.minigl.render();
    } else {
      gradient.start();
    }
  } catch (err) {
    // Brak WebGL (stare urządzenie)? Chowamy canvas — zostaje
    // zwykłe ciemne tło z siatką, strona działa dalej.
    console.warn("Tło gradientowe wyłączone:", err.message);
    canvas.remove();
  }
})();
