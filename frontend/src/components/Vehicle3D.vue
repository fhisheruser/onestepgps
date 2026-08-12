<script setup>
import { computed } from 'vue'

/**
 * A real 3D vehicle built from CSS `transform-style: preserve-3d` boxes.
 *
 * Deliberately dependency-free: a WebGL library would add hundreds of
 * kilobytes and a GPU context per card for what is, visually, a handful of
 * shaded rectangles. Each vehicle is a list of boxes positioned in model
 * space; the six faces of every box are absolutely positioned planes.
 */

const props = defineProps({
  type: { type: String, default: 'car' },
  color: { type: String, default: '#B4643C' },
  size: { type: Number, default: 120 },
  /** Yaw in degrees; wire this to a device heading to point the vehicle. */
  heading: { type: Number, default: -32 },
  spin: { type: Boolean, default: false },
  /** Adds a soft contact shadow — off inside dense list rows. */
  shadow: { type: Boolean, default: true },
})

// Model space is 100 units long; `size` scales the whole rig.
const PROFILES = {
  car: {
    boxes: [
      { w: 88, h: 20, d: 40, x: 0, y: 0, z: 0, tone: 'body' },
      { w: 48, h: 18, d: 36, x: -2, y: -18, z: 0, tone: 'cabin' },
      { w: 10, h: 8, d: 30, x: 44, y: -2, z: 0, tone: 'light' },
    ],
    wheels: [
      { x: -28, z: 21 },
      { x: 28, z: 21 },
      { x: -28, z: -21 },
      { x: 28, z: -21 },
    ],
    wheelSize: 18,
  },
  van: {
    boxes: [
      { w: 92, h: 38, d: 42, x: -4, y: -8, z: 0, tone: 'body' },
      { w: 26, h: 22, d: 40, x: 40, y: -16, z: 0, tone: 'cabin' },
      { w: 8, h: 8, d: 32, x: 50, y: -4, z: 0, tone: 'light' },
    ],
    wheels: [
      { x: -30, z: 22 },
      { x: 30, z: 22 },
      { x: -30, z: -22 },
      { x: 30, z: -22 },
    ],
    wheelSize: 18,
  },
  truck: {
    boxes: [
      { w: 30, h: 32, d: 42, x: 38, y: -14, z: 0, tone: 'cabin' },
      { w: 62, h: 40, d: 44, x: -20, y: -18, z: 0, tone: 'body' },
      { w: 6, h: 10, d: 34, x: 54, y: -6, z: 0, tone: 'light' },
      { w: 96, h: 8, d: 40, x: 0, y: 6, z: 0, tone: 'chassis' },
    ],
    wheels: [
      { x: 34, z: 23 },
      { x: -12, z: 23 },
      { x: -34, z: 23 },
      { x: 34, z: -23 },
      { x: -12, z: -23 },
      { x: -34, z: -23 },
    ],
    wheelSize: 20,
  },
  pickup: {
    boxes: [
      { w: 44, h: 24, d: 42, x: 22, y: -10, z: 0, tone: 'body' },
      { w: 34, h: 20, d: 38, x: 20, y: -28, z: 0, tone: 'cabin' },
      { w: 46, h: 16, d: 42, x: -26, y: -14, z: 0, tone: 'bed' },
      { w: 96, h: 8, d: 40, x: 0, y: 4, z: 0, tone: 'chassis' },
    ],
    wheels: [
      { x: 26, z: 22 },
      { x: -28, z: 22 },
      { x: 26, z: -22 },
      { x: -28, z: -22 },
    ],
    wheelSize: 19,
  },
  bus: {
    boxes: [
      { w: 100, h: 44, d: 42, x: 0, y: -18, z: 0, tone: 'body' },
      { w: 92, h: 12, d: 38, x: 0, y: -30, z: 0, tone: 'glass' },
      { w: 8, h: 8, d: 32, x: 52, y: -6, z: 0, tone: 'light' },
    ],
    wheels: [
      { x: 34, z: 22 },
      { x: -32, z: 22 },
      { x: 34, z: -22 },
      { x: -32, z: -22 },
    ],
    wheelSize: 18,
  },
  pin: {
    boxes: [{ w: 34, h: 34, d: 34, x: 0, y: -16, z: 0, tone: 'body' }],
    wheels: [],
    wheelSize: 0,
  },
}

const profile = computed(() => PROFILES[props.type] || PROFILES.car)
const scale = computed(() => props.size / 100)

/** Shade a hex colour by a signed percentage, for cheap per-face lighting. */
function shade(hex, percent) {
  const normalised = hex.replace('#', '')
  const full =
    normalised.length === 3
      ? normalised
          .split('')
          .map((c) => c + c)
          .join('')
      : normalised
  const value = Number.parseInt(full, 16)
  if (Number.isNaN(value)) return hex

  const clamp = (n) => Math.max(0, Math.min(255, Math.round(n)))
  const r = clamp(((value >> 16) & 255) * (1 + percent))
  const g = clamp(((value >> 8) & 255) * (1 + percent))
  const b = clamp((value & 255) * (1 + percent))
  return `rgb(${r}, ${g}, ${b})`
}

function toneColor(tone) {
  switch (tone) {
    case 'cabin':
      return shade(props.color, 0.22)
    case 'glass':
      return 'rgba(160, 200, 214, 0.85)'
    case 'light':
      return '#F6E7B8'
    case 'chassis':
      return '#3B322B'
    case 'bed':
      return shade(props.color, -0.22)
    default:
      return props.color
  }
}

/** Per-face brightness fakes a single light source from the upper front-left. */
const FACE_LIGHT = {
  top: 0.16,
  bottom: -0.4,
  front: 0.04,
  back: -0.3,
  left: -0.18,
  right: -0.06,
}

function faceStyle(box, face) {
  return {
    background: shade(toneColor(box.tone), FACE_LIGHT[face] ?? 0),
  }
}

function boxVars(box) {
  return {
    '--w': `${box.w}px`,
    '--h': `${box.h}px`,
    '--d': `${box.d}px`,
    transform: `translate3d(${box.x}px, ${box.y}px, ${box.z}px)`,
  }
}

function wheelVars(wheel) {
  const s = profile.value.wheelSize
  return {
    '--w': `${s}px`,
    '--h': `${s}px`,
    '--d': `${Math.round(s * 0.45)}px`,
    transform: `translate3d(${wheel.x}px, ${12}px, ${wheel.z}px)`,
  }
}

const rigStyle = computed(() => ({
  transform: `scale(${scale.value}) rotateX(-16deg) rotateY(${props.heading}deg)`,
}))

const FACES = ['front', 'back', 'left', 'right', 'top', 'bottom']
</script>

<template>
  <div
    class="scene-3d relative grid place-items-center"
    :style="{ width: `${size}px`, height: `${size * 0.72}px` }"
    aria-hidden="true"
  >
    <div class="preserve-3d rig" :class="{ 'animate-spin-slow': spin }" :style="rigStyle">
      <div v-for="(box, index) in profile.boxes" :key="`b-${index}`" class="preserve-3d box" :style="boxVars(box)">
        <span v-for="face in FACES" :key="face" class="face" :class="face" :style="faceStyle(box, face)" />
      </div>

      <div
        v-for="(wheel, index) in profile.wheels"
        :key="`w-${index}`"
        class="preserve-3d box wheel"
        :style="wheelVars(wheel)"
      >
        <span v-for="face in FACES" :key="face" class="face" :class="face" />
      </div>
    </div>

    <div
      v-if="shadow"
      class="pointer-events-none absolute bottom-1 left-1/2 -translate-x-1/2 rounded-[50%] bg-ink-900/20 blur-md dark:bg-black/50"
      :style="{ width: `${size * 0.62}px`, height: `${size * 0.1}px` }"
    />
  </div>
</template>

<style scoped>
.rig {
  transform-style: preserve-3d;
  will-change: transform;
  transition: transform 600ms cubic-bezier(0.22, 1, 0.36, 1);
}

.box {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 0;
  height: 0;
}

.face {
  position: absolute;
  left: 0;
  top: 0;
  display: block;
  backface-visibility: hidden;
}

/* Each face is a plane pushed out to half the box depth along its own axis. */
.face.front {
  width: var(--w);
  height: var(--h);
  transform: translate(-50%, -50%) translateZ(calc(var(--d) / 2));
}
.face.back {
  width: var(--w);
  height: var(--h);
  transform: translate(-50%, -50%) rotateY(180deg) translateZ(calc(var(--d) / 2));
}
.face.right {
  width: var(--d);
  height: var(--h);
  transform: translate(-50%, -50%) rotateY(90deg) translateZ(calc(var(--w) / 2));
}
.face.left {
  width: var(--d);
  height: var(--h);
  transform: translate(-50%, -50%) rotateY(-90deg) translateZ(calc(var(--w) / 2));
}
.face.top {
  width: var(--w);
  height: var(--d);
  transform: translate(-50%, -50%) rotateX(90deg) translateZ(calc(var(--h) / 2));
}
.face.bottom {
  width: var(--w);
  height: var(--d);
  transform: translate(-50%, -50%) rotateX(-90deg) translateZ(calc(var(--h) / 2));
}

.wheel .face {
  background: #241f1b;
  border-radius: 40%;
}
.wheel .face.left,
.wheel .face.right {
  background: #3a322c;
  border-radius: 50%;
  box-shadow: inset 0 0 0 3px #171310;
}
</style>
