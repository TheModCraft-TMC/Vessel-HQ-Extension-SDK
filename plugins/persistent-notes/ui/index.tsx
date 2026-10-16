import { useEffect, useState } from 'react';

import type {
  NativeExtensionProps,
  ExtensionNote,
} from '@/core/native-extensions/sdk';

import styles from './index.module.css';

/** A real server-persisted write-back example, isolated to its registration. */
export default function PersistentNotes({ storage }: NativeExtensionProps) {
  const [saved, setSaved] = useState<ExtensionNote>();
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('Loading notes…');
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let disposed = false;
    if (!storage) return;
    void storage
      .read()
      .then((note) => {
        if (disposed) return null;
        setSaved(note);
        setText(note.text);
        setMessage('Notes loaded.');
        return null;
      })
      .catch((error: unknown) => {
        if (!disposed)
          setMessage(
            error instanceof Error ? error.message : 'Unable to load notes.'
          );
      });
    return () => {
      disposed = true;
    };
  }, [storage, attempt]);

  async function save() {
    if (!storage || !saved || busy) return;
    setBusy(true);
    try {
      const note = await storage.write({ text, version: saved.version });
      setSaved(note);
      setText(note.text);
      setMessage(`Saved to Vessel. Version ${note.version}.`);
    } catch (error) {
      setMessage(
        error instanceof Error
          ? error.message
          : 'Unable to save. Reload to check for conflicting edits.'
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.root} aria-label="Persistent extension notes">
      <h2>Extension notes</h2>
      <p>
        Save a shared note for this extension in Vessel. Notes survive browser
        reloads. All administrators with this extension’s storage grants can
        read and edit them. Do not store secrets.
      </p>
      <label htmlFor="extension-note">Notes</label>
      <textarea
        id="extension-note"
        rows={12}
        maxLength={16384}
        value={text}
        disabled={!saved || busy}
        onChange={(event) => setText(event.target.value)}
      />
      <div className={styles.actions}>
        <button
          type="button"
          disabled={
            !saved ||
            busy ||
            text === saved.text ||
            new TextEncoder().encode(text).length > 16384
          }
          onClick={() => void save()}
        >
          {busy ? 'Saving…' : 'Save notes'}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            setSaved(undefined);
            setMessage('Loading notes…');
            setAttempt((value) => value + 1);
          }}
        >
          Reload saved notes (discard edits)
        </button>
      </div>
      <p role="status">
        {storage ? message : 'This host does not provide extension storage.'}
      </p>
    </section>
  );
}
