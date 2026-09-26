import { test, expect, roomCode } from '../lib/fixtures.js';

// A Firefox peer uses its built-in fake streams (moving colour bars + tone via
// media.navigator.streams.fake), so identity assertions stay one-directional:
// the Firefox side still decodes alice's real red frames.
test('a firefox peer connects and exchanges media with a chromium peer', async ({
  newPeer,
}) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const fox = await newPeer('fox', 'blue', 'firefox');

  await alice.join(code);
  await fox.join(code);

  await expect(alice.remoteVideo('fox')).toBeVisible();
  await expect(fox.remoteVideo('alice')).toBeVisible();

  // Both sides' peer connections settle, not just their tiles.
  for (const peer of [alice, fox]) {
    await expect
      .poll(async () => (await peer.connections()).map((c) => c.connectionState))
      .toEqual(['connected']);
  }

  // fox -> alice: bytes and frames flow (colour is firefox's own fake pattern).
  await expect
    .poll(async () => (await alice.inboundVideo('fox')).framesDecoded)
    .toBeGreaterThan(5);

  // alice -> fox: alice's red camera is identifiable on the firefox side.
  await expect
    .poll(async () => (await fox.inboundVideo('alice')).framesDecoded)
    .toBeGreaterThan(5);
  await expect
    .poll(async () => {
      const [r, g, b] = await fox.remotePixel('alice');
      return r - Math.max(g, b);
    })
    .toBeGreaterThan(50);

  // Audio flows in both directions.
  await expect.poll(async () => alice.inboundAudioEnergy('fox')).toBeGreaterThan(0);
  await expect.poll(async () => fox.inboundAudioEnergy('alice')).toBeGreaterThan(0);
});
