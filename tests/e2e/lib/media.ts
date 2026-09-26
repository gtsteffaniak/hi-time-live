import { mkdirSync, existsSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

/**
 * Chrome can be fed deterministic fake webcam/microphone input from files:
 *   --use-file-for-fake-video-capture=<y4m>
 *   --use-file-for-fake-audio-capture=<wav>
 * Both files loop forever, which makes every peer's media uniquely identifiable:
 * a solid colour frame and a single audio tone per peer. Tests can then assert
 * *which* peer's media arrived, not merely that bytes moved.
 */

export const FIXTURE_DIR = join(import.meta.dirname, '..', '.media');

export interface PeerMedia {
  /** rgb of every pixel of this peer's fake camera */
  rgb: [number, number, number];
  /** frequency in Hz of this peer's fake microphone tone */
  toneHz: number;
  videoFile: string;
  audioFile: string;
}

const WIDTH = 320;
const HEIGHT = 240;
const FPS = 30;
const SAMPLE_RATE = 48000;

function rgbToYuv([r, g, b]: [number, number, number]): [number, number, number] {
  // BT.601 studio swing, matching C420mpeg2 below.
  const y = 16 + (65.481 * r + 128.553 * g + 24.966 * b) / 255;
  const u = 128 + (-37.797 * r - 74.203 * g + 112.0 * b) / 255;
  const v = 128 + (112.0 * r - 93.786 * g - 18.214 * b) / 255;
  return [y, u, v].map((c) => Math.max(0, Math.min(255, Math.round(c)))) as [number, number, number];
}

function solidColourY4m(rgb: [number, number, number], frames = 2): Buffer {
  const [y, u, v] = rgbToYuv(rgb);
  const header = Buffer.from(`YUV4MPEG2 W${WIDTH} H${HEIGHT} F${FPS}:1 Ip A1:1 C420mpeg2\n`, 'ascii');
  const yPlane = Buffer.alloc(WIDTH * HEIGHT, y);
  const uPlane = Buffer.alloc((WIDTH * HEIGHT) / 4, u);
  const vPlane = Buffer.alloc((WIDTH * HEIGHT) / 4, v);
  const frame = Buffer.concat([Buffer.from('FRAME\n', 'ascii'), yPlane, uPlane, vPlane]);
  return Buffer.concat([header, ...Array.from({ length: frames }, () => frame)]);
}

function sineWav(toneHz: number, seconds = 2): Buffer {
  const samples = SAMPLE_RATE * seconds;
  const data = Buffer.alloc(samples * 2);
  for (let i = 0; i < samples; i++) {
    const amplitude = Math.sin((2 * Math.PI * toneHz * i) / SAMPLE_RATE);
    data.writeInt16LE(Math.round(amplitude * 0.8 * 32767), i * 2);
  }
  const header = Buffer.alloc(44);
  header.write('RIFF', 0, 'ascii');
  header.writeUInt32LE(36 + data.length, 4);
  header.write('WAVE', 8, 'ascii');
  header.write('fmt ', 12, 'ascii');
  header.writeUInt32LE(16, 16); // PCM chunk size
  header.writeUInt16LE(1, 20); // PCM
  header.writeUInt16LE(1, 22); // mono
  header.writeUInt32LE(SAMPLE_RATE, 24);
  header.writeUInt32LE(SAMPLE_RATE * 2, 28); // byte rate
  header.writeUInt16LE(2, 32); // block align
  header.writeUInt16LE(16, 34); // bits per sample
  header.write('data', 36, 'ascii');
  header.writeUInt32LE(data.length, 40);
  return Buffer.concat([header, data]);
}

/** Writes (once) and returns the fake camera/microphone files for a named peer. */
export function mediaFor(name: string, rgb: [number, number, number], toneHz: number): PeerMedia {
  mkdirSync(FIXTURE_DIR, { recursive: true });
  const videoFile = join(FIXTURE_DIR, `${name}.y4m`);
  const audioFile = join(FIXTURE_DIR, `${name}.wav`);
  if (!existsSync(videoFile)) writeFileSync(videoFile, solidColourY4m(rgb));
  if (!existsSync(audioFile)) writeFileSync(audioFile, sineWav(toneHz));
  return { rgb, toneHz, videoFile, audioFile };
}

export const MEDIA = {
  red: () => mediaFor('red', [220, 20, 20], 440),
  green: () => mediaFor('green', [20, 200, 20], 660),
  blue: () => mediaFor('blue', [20, 20, 220], 880),
  yellow: () => mediaFor('yellow', [220, 220, 20], 1100),
} satisfies Record<string, () => PeerMedia>;

export type MediaName = keyof typeof MEDIA;
