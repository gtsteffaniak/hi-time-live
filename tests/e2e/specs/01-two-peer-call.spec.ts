import { test, expect, roomCode } from '../lib/fixtures.js';

/**
 * The golden path: two participants in one room must end up with a connected
 * RTCPeerConnection each, and must actually receive *the other peer's* camera
 * and microphone - verified by pixel colour and audio tone, not just byte counts.
 */
test('two peers connect and exchange real audio and video', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await expect(alice.page.locator('#status-text')).toHaveText(/Waiting on others to join/);

  await bob.join(code);

  // Both sides render a tile for the other participant.
  await expect(alice.remoteVideo('bob')).toBeVisible();
  await expect(bob.remoteVideo('alice')).toBeVisible();

  // Both peer connections reach a connected ICE state.
  for (const peer of [alice, bob]) {
    await expect
      .poll(async () => (await peer.connections()).map((c) => c.connectionState), {
        message: `${peer.opts.name} peer connections never connected`,
      })
      .toEqual(['connected']);
  }

  // Video really decodes on both sides.
  await expect.poll(async () => (await alice.inboundVideo('bob')).framesDecoded).toBeGreaterThan(5);
  await expect.poll(async () => (await bob.inboundVideo('alice')).framesDecoded).toBeGreaterThan(5);

  // Audio really arrives on both sides.
  await expect.poll(async () => alice.inboundAudioEnergy('bob')).toBeGreaterThan(0);
  await expect.poll(async () => bob.inboundAudioEnergy('alice')).toBeGreaterThan(0);

  // Identity: alice's tile for bob shows bob's blue camera, and vice versa.
  const bobPixelOnAlice = await alice.remotePixel('bob');
  expect(bobPixelOnAlice[2], `expected blue-dominant pixel, got ${bobPixelOnAlice}`).toBeGreaterThan(
    Math.max(bobPixelOnAlice[0], bobPixelOnAlice[1]) + 40,
  );
  const alicePixelOnBob = await bob.remotePixel('alice');
  expect(alicePixelOnBob[0], `expected red-dominant pixel, got ${alicePixelOnBob}`).toBeGreaterThan(
    Math.max(alicePixelOnBob[1], alicePixelOnBob[2]) + 40,
  );

  // Identity of audio: each side hears the other's tone, not its own.
  expect(await alice.remoteToneHz('bob')).toBeCloseTo(bob.media.toneHz, -2);
  expect(await bob.remoteToneHz('alice')).toBeCloseTo(alice.media.toneHz, -2);

  expect(alice.pageErrors, 'alice had uncaught page errors').toEqual([]);
  expect(bob.pageErrors, 'bob had uncaught page errors').toEqual([]);
});

test('a peer leaving removes its tile from the remaining participant', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await bob.join(code);
  await expect(alice.remoteVideo('bob')).toBeVisible();

  await bob.leave();

  await expect(alice.page.locator(alice.tile('bob'))).toHaveCount(0);
  await expect(alice.page.locator('#status-text')).toHaveText(/Waiting on others to join/);
});
