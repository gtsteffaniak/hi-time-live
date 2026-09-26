import { test, expect, roomCode } from '../lib/fixtures.js';

test('a peer that reloads the page rejoins and streams again', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  await bob.join(code);
  await expect(alice.remoteVideo('bob')).toBeVisible();

  // A reload drops the SSE stream and comes back with a brand new user id.
  await bob.page.reload();
  await expect(alice.page.locator(alice.tile('bob'))).toHaveCount(0);

  await bob.page.fill('#nameInput', 'bob');
  await bob.page.click('#start-button');

  await expect(alice.remoteVideo('bob')).toBeVisible();
  await expect
    .poll(async () => (await alice.inboundVideo('bob')).framesDecoded)
    .toBeGreaterThan(5);
});

test('signalling failures surface instead of silently hanging', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  // Every signalling POST from bob fails: the call must not be reported as up.
  await bob.page.route('**/event', (route) => route.abort('failed'));

  await alice.join(code);
  await bob.join(code);

  await expect(alice.page.locator('#status-text')).toBeVisible();
  await expect
    .poll(async () => (await alice.inboundVideo('bob').catch(() => ({ framesDecoded: 0 }))).framesDecoded, {
      timeout: 20_000,
    })
    .toBe(0);
  // The loading modal must stay up rather than showing an empty, "connected" room.
  await expect(alice.page.locator('#loadingModal')).not.toHaveClass(/hidden/);
});

test('the server rejects signalling from a client that is not in the room', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  await alice.join(code);

  const status = await alice.page.evaluate(async (roomId) => {
    const res = await fetch('/event', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ eventType: 'newOffer', userId: 'intruder__0000', code: roomId, offer: 'v=0' }),
    });
    return res.status;
  }, code);

  expect(status).toBe(403);
  await expect(alice.page.locator('#video-container video')).toHaveCount(0);
});
