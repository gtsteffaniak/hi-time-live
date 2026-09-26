import { test, expect, roomCode } from '../lib/fixtures.js';

test('muting stops the remote side receiving audio energy', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await bob.join(code);
  await expect(alice.remoteVideo('bob')).toBeVisible();
  await expect.poll(async () => alice.inboundAudioEnergy('bob')).toBeGreaterThan(0);

  await bob.page.click('#muteButton');
  await expect(bob.page.locator('#muteButton')).toHaveText('Unmute');

  // Energy is cumulative, so assert that it stops growing rather than resets.
  await alice.page.waitForTimeout(1000);
  const settled = await alice.inboundAudioEnergy('bob');
  await alice.page.waitForTimeout(2000);
  const after = await alice.inboundAudioEnergy('bob');
  expect(after - settled).toBeLessThan(1e-5);

  await bob.page.click('#muteButton');
  await expect(bob.page.locator('#muteButton')).toHaveText('Mute');
  await expect.poll(async () => alice.inboundAudioEnergy('bob')).toBeGreaterThan(after);
});

// A disabled track keeps the RTP stream alive and sends black frames, so the
// assertion is about what the remote side sees, not about the frame counter.
test('disabling video blacks out the remote tile and re-enabling restores it', async ({
  newPeer,
}) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await bob.join(code);
  await expect.poll(async () => (await alice.inboundVideo('bob')).framesDecoded).toBeGreaterThan(5);
  expect(Math.max(...(await alice.remotePixel('bob')))).toBeGreaterThan(60);

  await bob.page.click('#vidButton');
  await expect(bob.page.locator('#vidButton')).toHaveText('Enable Video');
  await expect
    .poll(async () => Math.max(...(await alice.remotePixel('bob'))), {
      message: "bob's tile never went black after disabling video",
    })
    .toBeLessThan(25);

  await bob.page.click('#vidButton');
  await expect(bob.page.locator('#vidButton')).toHaveText('Disable Video');
  await expect
    .poll(async () => Math.max(...(await alice.remotePixel('bob'))))
    .toBeGreaterThan(60);
});
