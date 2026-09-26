import { test, expect, roomCode } from '../lib/fixtures.js';

test('participants in different rooms never see each other', async ({ newPeer }) => {
  const roomA = roomCode();
  const roomB = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');
  const mallory = await newPeer('mallory', 'yellow');

  await alice.join(roomA);
  await bob.join(roomA);
  await mallory.join(roomB);

  await expect(alice.remoteVideo('bob')).toBeVisible();

  // Room A must be unaware of room B, and room B must stay empty.
  await expect(alice.page.locator(alice.tile('mallory'))).toHaveCount(0);
  await expect(bob.page.locator(bob.tile('mallory'))).toHaveCount(0);
  await expect(mallory.page.locator('#video-container video')).toHaveCount(0);
  expect(await mallory.connections()).toEqual([]);
  await expect(mallory.page.locator('#status-text')).toHaveText(/Waiting on others to join/);
});

test('an invalid room code renders the invalid-room page, not a broken room', async ({
  newPeer,
}) => {
  const peer = await newPeer('alice', 'red');
  const response = await peer.page.goto('/room?id=not-a-uuid');

  expect(response?.status()).toBe(404);
  await expect(peer.page.locator('body')).toContainText(/not a valid room/i);
  await expect(peer.page.locator('#nameInput')).toHaveCount(0);
  expect(peer.pageErrors).toEqual([]);
});
