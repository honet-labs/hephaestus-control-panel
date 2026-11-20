import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import ts from 'typescript';

// Compile the real helper without adding a test framework/runtime dependency.
const source = readFileSync(new URL('../src/lib/terminalClipboard.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText;
const { handleTerminalClipboardShortcut: handle } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const flush = () => new Promise((resolve) => setImmediate(resolve));

function setup(overrides = {}) {
  const result = { copied: [], pasted: [], notices: [], reads: 0 };
  const actions = {
    getSelection: () => 'selected output',
    writeText: async (text) => { result.copied.push(text); },
    readText: async () => { result.reads++; return 'echo test\n'; },
    canPaste: () => true,
    paste: (text) => result.pasted.push(text),
    notify: (...args) => result.notices.push(args),
    ...overrides,
  };
  return { actions, result };
}

function event(key, overrides = {}) {
  return { key, ctrlKey: true, shiftKey: true, altKey: false, metaKey: false, isComposing: false, repeat: false, type: 'keydown', prevented: false, stopped: false,
    preventDefault() { this.prevented = true; }, stopPropagation() { this.stopped = true; }, ...overrides };
}

test('Ctrl+Shift+C cancels browser inspect synchronously and copies selected output only', async () => {
  const { actions, result } = setup();
  const key = event('C');
  assert.equal(handle(key, actions), true);
  assert.equal(key.prevented, true);
  assert.equal(key.stopped, true);
  await flush();
  assert.deepEqual(result.copied, ['selected output']);
  assert.deepEqual(result.pasted, []);
});

test('Ctrl+Shift+V pastes exactly once through the terminal paste callback', async () => {
  const { actions, result } = setup();
  const key = event('V');
  assert.equal(handle(key, actions), true);
  assert.equal(key.prevented, true);
  handle(event('V', { type: 'keyup' }), actions);
  handle(event('V', { repeat: true }), actions);
  await flush();
  assert.equal(result.reads, 1);
  assert.deepEqual(result.pasted, ['echo test\n']);
});

test('plain Ctrl+C, Ctrl+V, Cmd+C/V and other shortcuts retain their existing behavior', () => {
  const { actions, result } = setup();
  for (const key of [event('c', { shiftKey: false }), event('v', { shiftKey: false }), event('c', { ctrlKey: false, metaKey: true }), event('v', { ctrlKey: false, metaKey: true }), event('i'), event('c', { altKey: true }), event('c', { isComposing: true })]) {
    assert.equal(handle(key, actions), false);
    assert.equal(key.prevented, false);
    assert.equal(key.stopped, false);
  }
  assert.deepEqual(result.copied, []);
  assert.deepEqual(result.pasted, []);
});

test('copy with no selection never copies the full screen or sends shell input', async () => {
  const { actions, result } = setup({ getSelection: () => '' });
  const key = event('c');
  assert.equal(handle(key, actions), true);
  assert.equal(key.prevented, true);
  await flush();
  assert.deepEqual(result.copied, []);
  assert.deepEqual(result.pasted, []);
  assert.equal(result.notices[0][1], 'error');
});

test('disconnected terminal does not even read clipboard', async () => {
  const { actions, result } = setup({ canPaste: () => false });
  handle(event('v'), actions);
  await flush();
  assert.equal(result.reads, 0);
  assert.deepEqual(result.pasted, []);
});

test('tab, focus, or socket change during clipboard permission prompt cancels paste', async () => {
  let ready = true;
  let resolve;
  const { actions, result } = setup({ canPaste: () => ready, readText: () => new Promise((done) => { resolve = done; }) });
  handle(event('v'), actions);
  ready = false;
  resolve('must not go to a different shell');
  await flush();
  assert.deepEqual(result.pasted, []);
  assert.equal(result.notices[0][1], 'error');
});

test('denied or unavailable clipboard APIs report an error with no fallback shell input', async () => {
  for (const key of ['c', 'v']) {
    const unavailable = () => { throw new Error('clipboard unavailable'); };
    const { actions, result } = setup({ writeText: unavailable, readText: unavailable });
    const press = event(key);
    handle(press, actions);
    assert.equal(press.prevented, true);
    await flush();
    assert.equal(result.notices[0][1], 'error');
    assert.deepEqual(result.pasted, []);
  }
});
