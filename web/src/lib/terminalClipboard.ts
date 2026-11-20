interface ClipboardActions {
  getSelection(): string;
  writeText(text: string): Promise<void>;
  readText(): Promise<string>;
  canPaste(): boolean;
  paste(text: string): void;
  notify(text: string, type: 'success' | 'error'): void;
}

// Called only by the focused xterm instance, not a document-wide DevTools block.
// Consume synchronously, before clipboard permissions/Promises resolve, so the
// browser shortcut and xterm's normal input path cannot also handle this event.
export function handleTerminalClipboardShortcut(event: KeyboardEvent, actions: ClipboardActions): boolean {
  const key = event.key.toLowerCase();
  if (!event.ctrlKey || !event.shiftKey || event.altKey || event.metaKey || event.isComposing || (key !== 'c' && key !== 'v')) return false;
  event.preventDefault();
  event.stopPropagation();
  if (event.type !== 'keydown' || event.repeat) return true;

  if (key === 'c') {
    const selected = actions.getSelection();
    if (!selected) {
      actions.notify('Select terminal text before copying.', 'error');
      return true;
    }
    void (async () => {
      try {
        await actions.writeText(selected);
        actions.notify('Terminal selection copied.', 'success');
      } catch {
        actions.notify('Clipboard copy unavailable. Use HTTPS and allow clipboard access in your browser.', 'error');
      }
    })();
  } else {
    if (!actions.canPaste()) {
      actions.notify('Reconnect the terminal before pasting.', 'error');
      return true;
    }
    void (async () => {
      try {
        const text = await actions.readText();
        // A clipboard permission prompt can outlive a tab/socket change. Never
        // send its result to a different shell or a no-longer-focused terminal.
        if (!actions.canPaste()) {
          actions.notify('Terminal changed while reading clipboard. Paste again in the intended terminal.', 'error');
          return;
        }
        if (text) actions.paste(text);
      } catch {
        actions.notify('Clipboard paste unavailable. Use HTTPS and allow clipboard access, or try Ctrl+V.', 'error');
      }
    })();
  }
  return true;
}
