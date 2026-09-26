import { test, expect, roomCode } from '../lib/fixtures.js';

// Two tabs can share a display name but always get distinct user ids. Both are
// full participants: a second remote sees two tiles, and closing one tab leaves
// the other connected.
test('a duplicate same-name tab is a second participant, not a replacement', async ({
  newPeer,
}) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await bob.join(code);
  await expect(bob.remoteVideo('alice')).toBeVisible();

  const secondTab = await alice.joinSecondTab(code);
  await expect.poll(async () => bob.remoteTileCount('alice')).toBe(2);

  await secondTab.close();
  await expect.poll(async () => bob.remoteTileCount('alice')).toBe(1);

  // The surviving tab is unaffected: media still flows and the connection stays up.
  await expect
    .poll(async () => (await bob.inboundVideo('alice')).framesDecoded)
    .toBeGreaterThan(5);
  await expect
    .poll(async () =>
      (await bob.connections()).filter((c) => c.peerId.startsWith('alice__')).length,
    )
    .toBe(1);
});

// Mute and video-off are broadcast as mediaState and must reach every remote,
// not just one: both tiles go quiet/black and both overlays are labelled.
test('muting and disabling video propagate to every remote peer', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');
  const carol = await newPeer('carol', 'green');

  await alice.join(code);
  await bob.join(code);
  await carol.join(code);
  for (const observer of [alice, carol]) {
    await expect.poll(async () => observer.inboundAudioEnergy('bob')).toBeGreaterThan(0);
  }

  await bob.page.click('#muteButton');
  for (const observer of [alice, carol]) {
    await expect.poll(async () => observer.remoteOverlayText('bob')).toContain('muted');
    await observer.page.waitForTimeout(1000);
    const settled = await observer.inboundAudioEnergy('bob');
    await observer.page.waitForTimeout(2000);
    expect((await observer.inboundAudioEnergy('bob')) - settled).toBeLessThan(1e-5);
  }

  await bob.page.click('#vidButton');
  for (const observer of [alice, carol]) {
    await expect.poll(async () => observer.remoteOverlayText('bob')).toContain('video off');
    await expect
      .poll(async () => Math.max(...(await observer.remotePixel('bob'))), {
        message: "bob's tile never went black",
      })
      .toBeLessThan(25);
  }

  // Restoring clears both flags on both remotes.
  await bob.page.click('#muteButton');
  await bob.page.click('#vidButton');
  for (const observer of [alice, carol]) {
    await expect.poll(async () => observer.remoteOverlayText('bob')).toBe('bob');
    await expect.poll(async () => observer.inboundAudioEnergy('bob')).toBeGreaterThan(0);
    await expect
      .poll(async () => Math.max(...(await observer.remotePixel('bob'))))
      .toBeGreaterThan(60);
  }
});
