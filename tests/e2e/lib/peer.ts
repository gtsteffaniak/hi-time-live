import { chromium, type Browser, type BrowserContext, type Page } from '@playwright/test';
import { MEDIA, type MediaName, type PeerMedia } from './media.js';

export interface PeerOptions {
  /** display name typed into the join modal; also the prefix of the signalling user id */
  name: string;
  /** which deterministic fake camera/microphone this peer publishes */
  media: MediaName;
  baseURL: string;
  headless?: boolean;
}

/**
 * `signaling.js` declares its peer-connection registry with a top-level `let`,
 * so it is a global lexical binding rather than a property of `window`. Page
 * functions below therefore reference it as a free variable.
 */
declare const pcs: Record<string, RTCPeerConnection | null>;

export interface InboundVideoStats {
  framesDecoded: number;
  bytesReceived: number;
}

export interface ConnectionState {
  peerId: string;
  connectionState: string;
  iceConnectionState: string;
  signalingState: string;
}

/**
 * A single conferencing participant, running in its own Chrome process so that
 * it can be given its own fake camera and microphone (those are process-wide
 * command line flags, not per-context options).
 */
export class Peer {
  readonly consoleLog: string[] = [];
  readonly pageErrors: string[] = [];
  readonly media: PeerMedia;
  private browser!: Browser;
  private context!: BrowserContext;
  page!: Page;

  private constructor(readonly opts: PeerOptions) {
    this.media = MEDIA[opts.media]();
  }

  static async launch(opts: PeerOptions): Promise<Peer> {
    const peer = new Peer(opts);
    peer.browser = await chromium.launch({
      headless: opts.headless ?? true,
      args: [
        '--use-fake-device-for-media-stream',
        '--use-fake-ui-for-media-stream',
        `--use-file-for-fake-video-capture=${peer.media.videoFile}`,
        `--use-file-for-fake-audio-capture=${peer.media.audioFile}`,
        '--autoplay-policy=no-user-gesture-required',
        '--disable-features=WebRtcHideLocalIpsWithMdns',
      ],
    });
    peer.context = await peer.browser.newContext({
      baseURL: opts.baseURL,
      permissions: ['camera', 'microphone'],
    });
    peer.page = await peer.context.newPage();
    peer.page.on('console', (m) => peer.consoleLog.push(`[${m.type()}] ${m.text()}`));
    peer.page.on('pageerror', (e) => peer.pageErrors.push(String(e)));
    return peer;
  }

  /** Opens the room and completes the privacy/name modal. */
  async join(roomCode: string): Promise<void> {
    await this.page.goto(`/room?id=${roomCode}`);
    await this.page.fill('#nameInput', this.opts.name);
    await this.page.click('#start-button');
    await this.page.waitForFunction(() => {
      const v = document.getElementById('localVideo') as HTMLVideoElement | null;
      return !!v?.srcObject;
    });
  }

  /** CSS selector for the tile rendered for a remote participant. */
  tile(remoteName: string): string {
    return `div[id^="${remoteName}__"][id$="-container"]`;
  }

  remoteVideo(remoteName: string) {
    return this.page.locator(`${this.tile(remoteName)} video`);
  }

  /** Signalling-level view of this peer's RTCPeerConnections. */
  async connections(): Promise<ConnectionState[]> {
    return this.page.evaluate(() =>
      Object.entries(pcs)
        .filter(([, pc]) => pc !== null)
        .map(([peerId, pc]) => ({
          peerId,
          connectionState: pc!.connectionState,
          iceConnectionState: pc!.iceConnectionState,
          signalingState: pc!.signalingState,
        })),
    );
  }

  async inboundVideo(remoteName: string): Promise<InboundVideoStats> {
    return this.page.evaluate(async (prefix) => {
      const entry = Object.entries(pcs).find(([id, pc]) => id.startsWith(prefix + '__') && pc);
      if (!entry) throw new Error(`no peer connection for ${prefix}`);
      const stats = await entry[1]!.getStats();
      const out = { framesDecoded: 0, bytesReceived: 0 };
      stats.forEach((r) => {
        if (r.type === 'inbound-rtp' && r.kind === 'video') {
          out.framesDecoded = r.framesDecoded ?? 0;
          out.bytesReceived = r.bytesReceived ?? 0;
        }
      });
      return out;
    }, remoteName);
  }

  /** Cumulative received audio energy, the standard "is audio actually flowing" signal. */
  async inboundAudioEnergy(remoteName: string): Promise<number> {
    return this.page.evaluate(async (prefix) => {
      const entry = Object.entries(pcs).find(([id, pc]) => id.startsWith(prefix + '__') && pc);
      if (!entry) throw new Error(`no peer connection for ${prefix}`);
      const stats = await entry[1]!.getStats();
      let energy = 0;
      stats.forEach((r) => {
        if (r.type === 'inbound-rtp' && r.kind === 'audio') energy = r.totalAudioEnergy ?? 0;
      });
      return energy;
    }, remoteName);
  }

  /** Centre pixel of the remote tile: identifies *whose* camera is rendered there. */
  async remotePixel(remoteName: string): Promise<[number, number, number]> {
    return this.page.evaluate((selector) => {
      const video = document.querySelector(`${selector} video`) as HTMLVideoElement | null;
      if (!video) throw new Error(`no remote video for ${selector}`);
      if (!video.videoWidth) throw new Error('remote video has no frames yet');
      const canvas = document.createElement('canvas');
      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;
      const ctx = canvas.getContext('2d')!;
      ctx.drawImage(video, 0, 0);
      const d = ctx.getImageData(canvas.width >> 1, canvas.height >> 1, 1, 1).data;
      return [d[0], d[1], d[2]] as [number, number, number];
    }, this.tile(remoteName));
  }

  /** Dominant frequency of the remote audio track: identifies *whose* microphone is heard. */
  async remoteToneHz(remoteName: string): Promise<number> {
    return this.page.evaluate(async (selector) => {
      const video = document.querySelector(`${selector} video`) as HTMLVideoElement | null;
      const stream = video?.srcObject as MediaStream | null;
      const track = stream?.getAudioTracks()[0];
      if (!track) throw new Error(`no remote audio track for ${selector}`);
      const ctx = new AudioContext();
      const analyser = ctx.createAnalyser();
      analyser.fftSize = 8192;
      ctx.createMediaStreamSource(new MediaStream([track])).connect(analyser);
      const bins = new Float32Array(analyser.frequencyBinCount);
      let best = { hz: 0, db: -Infinity };
      // Average a few reads so a single silent frame cannot decide the result.
      for (let i = 0; i < 10; i++) {
        await new Promise((r) => setTimeout(r, 100));
        analyser.getFloatFrequencyData(bins);
        for (let b = 0; b < bins.length; b++) {
          if (bins[b] > best.db) best = { hz: (b * ctx.sampleRate) / analyser.fftSize, db: bins[b] };
        }
      }
      await ctx.close();
      return best.hz;
    }, this.tile(remoteName));
  }

  async statusText(): Promise<string> {
    return (await this.page.locator('#status-text').innerText()).trim();
  }

  /** Simulates the user closing the tab (SSE drops, server should notice). */
  async leave(): Promise<void> {
    await this.page.close();
  }

  async dispose(): Promise<void> {
    await this.context.close().catch(() => {});
    await this.browser.close().catch(() => {});
  }
}

export function roomCode(): string {
  return crypto.randomUUID();
}
