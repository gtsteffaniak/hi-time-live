import { test, expect, roomCode } from '../lib/fixtures.js';

// The TURN relay used by the positive test. Set by `make test-e2e-turn` and by
// CI (see .github/workflows/pr.yaml); without it the relay test is skipped.
const TURN_URL = process.env.HITIME_TURN_URL;

// relay-only ICE with TURN stripped is the deterministic stand-in for "this
// user's NAT/firewall admits no direct path": host and srflx candidates are
// unusable, no relay exists, so no candidate pair can ever succeed. The short
// connect timeout keeps the watchdog from making the test slow.
const noNetworkPath = { relay: '1', noturn: '1', connecttimeout: '6000' };

/**
 * The failure this suite previously could not catch: signaling completes, but
 * the peers never establish a media path. It must be reported, not swallowed.
 */
test('a peer with no usable network path is surfaced as failed, not silently dropped', async ({
  newPeer,
}) => {
  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  await alice.join(code, { query: noNetworkPath });
  await bob.join(code, { query: noNetworkPath });

  // No media can flow over a connection that never comes up.
  await expect
    .poll(async () => (await alice.inboundVideo('bob').catch(() => ({ framesDecoded: 0 }))).framesDecoded, {
      timeout: 30_000,
      message: 'media flowed over a connection that should have no usable path',
    })
    .toBe(0);

  // The failure is visible as a red status row with a retry, and the room does
  // not silently collapse back to the "waiting on others" prompt: the failure
  // replaced the spinner, so the modal stays dismissed.
  await expect(alice.page.locator('#connection-bar .conn-failed')).toBeVisible({ timeout: 30_000 });
  await expect(alice.page.locator('#connection-bar .conn-retry')).toBeVisible();
  await expect(alice.page.locator('#loadingModal')).toHaveClass(/hidden/);

  expect(alice.pageErrors, 'alice had uncaught page errors').toEqual([]);
  expect(bob.pageErrors, 'bob had uncaught page errors').toEqual([]);
});

/**
 * The fix for the failure above: with a relay configured, a peer that can only
 * use a relay candidate must connect. This is the end-to-end proof that the
 * TURN wiring reaches the browser and is actually used.
 */
test('relay-only peers connect through the configured TURN server', async ({ newPeer }) => {
  test.skip(!TURN_URL, 'set HITIME_TURN_URL (see `make test-e2e-turn`) to run the TURN relay test');

  const code = roomCode();
  const alice = await newPeer('alice', 'red');
  const bob = await newPeer('bob', 'blue');

  // relay-only: host and srflx candidates are unusable, so the call can only
  // come up if the configured TURN server really relays.
  await alice.join(code, { query: { relay: '1' } });
  await bob.join(code, { query: { relay: '1' } });

  expect(await alice.iceServers(), 'the TURN server never reached the client').toContain(TURN_URL);

  await expect(alice.remoteVideo('bob')).toBeVisible({ timeout: 30_000 });
  await expect
    .poll(async () => (await alice.inboundVideo('bob')).framesDecoded, { timeout: 30_000 })
    .toBeGreaterThan(0);
  await expect
    .poll(async () => (await alice.connections()).map((c) => c.connectionState), { timeout: 30_000 })
    .toEqual(['connected']);
});
