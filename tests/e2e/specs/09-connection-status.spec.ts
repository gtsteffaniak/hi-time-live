import { test, expect, roomCode } from '../lib/fixtures.js';

/**
 * The status bar is the room's live roster. The regression it guards against is
 * that it used to disappear the instant a peer connected, so the user never saw
 * the room's state change. Rows must survive the transition to connected and be
 * removed only when the peer actually leaves.
 */
test('the status bar tracks a peer through connect and leave', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code);
  // Nobody else yet, so the bar has nothing to show.
  await expect(alice.page.locator('#connection-bar')).toHaveClass(/hidden/);

  await bob.join(code);

  // The row must remain visible once bob has settled, marked connected.
  await expect
    .poll(async () => (await alice.connectionBar()).map((row) => row.state), { timeout: 20_000 })
    .toEqual(['conn-connected']);
  await expect(alice.page.locator('#connection-bar')).not.toHaveClass(/hidden/);
  await expect(alice.page.locator('#connection-bar .conn-name')).toHaveText('bob');
  await expect(alice.page.locator('#connection-bar .conn-state')).toHaveText('connected');

  // Bob leaving removes his row, and with no peers left the bar hides again.
  await bob.leave();
  await expect(alice.page.locator('#connection-bar')).toHaveClass(/hidden/);
  await expect(alice.page.locator('#connection-bar .conn-row')).toHaveCount(0);
});

/**
 * A peer that has not connected keeps a row too, so a room mid-transition is
 * never blank. Blocking bob's signaling holds the handshake in "connecting"
 * deterministically (alice offers, bob can never answer).
 */
test('a peer that is still connecting is shown as a pulsing row', async ({ newPeer }) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await bob.page.route('**/event', (route) => route.abort('failed'));

  await alice.join(code);
  await bob.join(code);

  await expect
    .poll(async () => (await alice.connectionBar()).map((row) => row.state), { timeout: 20_000 })
    .toEqual(['conn-connecting']);
  await expect(alice.page.locator('#connection-bar')).not.toHaveClass(/hidden/);
  await expect(alice.page.locator('#connection-bar .conn-state')).toHaveText('connecting');
  await expect(alice.page.locator('#connection-bar .conn-name')).toHaveText('bob');
});
