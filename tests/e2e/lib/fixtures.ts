import { test as base, expect } from '@playwright/test';
import { Peer, type PeerOptions } from './peer.js';
import type { MediaName } from './media.js';

export interface Fixtures {
  /** Launches a participant in its own browser; torn down automatically. */
  newPeer: (name: string, media: MediaName) => Promise<Peer>;
}

export const test = base.extend<Fixtures>({
  newPeer: async ({ baseURL }, use) => {
    const peers: Peer[] = [];
    await use(async (name: string, media: MediaName) => {
      const opts: PeerOptions = { name, media, baseURL: baseURL! };
      const peer = await Peer.launch(opts);
      peers.push(peer);
      return peer;
    });
    for (const peer of peers) await peer.dispose();
  },
});

export { expect, Peer };
export { roomCode } from './peer.js';
