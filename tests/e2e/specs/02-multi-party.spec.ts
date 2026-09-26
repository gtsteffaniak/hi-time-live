import { test, expect, roomCode, type Peer } from '../lib/fixtures.js';

const DOMINANT: Record<string, 0 | 1 | 2> = { red: 0, green: 1, blue: 2, yellow: 0 };

async function expectSeesCamera(viewer: Peer, subject: Peer) {
  await expect(viewer.remoteVideo(subject.opts.name)).toBeVisible();
  await expect
    .poll(async () => (await viewer.inboundVideo(subject.opts.name)).framesDecoded, {
      message: `${viewer.opts.name} decoded no frames from ${subject.opts.name}`,
    })
    .toBeGreaterThan(5);
  const pixel = await viewer.remotePixel(subject.opts.name);
  const channel = DOMINANT[subject.opts.media];
  const others = pixel.filter((_, i) => i !== channel);
  expect(
    pixel[channel],
    `${viewer.opts.name} shows ${pixel} in ${subject.opts.name}'s tile, expected ${subject.opts.media}`,
  ).toBeGreaterThan(Math.max(...others) + 40);
}

test('three peers form a full mesh and each sees both others', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');
  const carol = await newPeer('carol', 'green');

  await alice.join(code);
  await bob.join(code);
  await expect(alice.remoteVideo('bob')).toBeVisible();

  await carol.join(code);

  await expectSeesCamera(alice, bob);
  await expectSeesCamera(alice, carol);
  await expectSeesCamera(bob, alice);
  await expectSeesCamera(bob, carol);
  await expectSeesCamera(carol, alice);
  await expectSeesCamera(carol, bob);

  for (const peer of [alice, bob, carol]) {
    await expect
      .poll(async () => (await peer.connections()).map((c) => c.connectionState).sort())
      .toEqual(['connected', 'connected']);
  }
});

test('one peer leaving a three-way call leaves the other two connected', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');
  const carol = await newPeer('carol', 'green');

  await alice.join(code);
  await bob.join(code);
  await carol.join(code);
  await expect(alice.remoteVideo('carol')).toBeVisible();

  await carol.leave();

  await expect(alice.page.locator(alice.tile('carol'))).toHaveCount(0);
  await expect(bob.page.locator(bob.tile('carol'))).toHaveCount(0);

  // The surviving pair must keep streaming.
  const before = (await alice.inboundVideo('bob')).framesDecoded;
  await expect
    .poll(async () => (await alice.inboundVideo('bob')).framesDecoded)
    .toBeGreaterThan(before);
});
