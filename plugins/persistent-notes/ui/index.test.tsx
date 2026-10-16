import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi } from 'vitest';

import type { NativeExtensionProps } from '@/core/native-extensions/sdk';

import PersistentNotes from './index';

function fixture() {
  const storage = {
    read: vi.fn().mockResolvedValue({ text: 'Saved note', version: 3 }),
    write: vi.fn().mockResolvedValue({ text: 'Updated note', version: 4 }),
  };
  const props: NativeExtensionProps = {
    storage,
    context: { environmentId: 1, stackId: 1, stackName: 'demo' },
    invoke: vi.fn(),
  };
  return { storage, props };
}

it('loads saved notes, writes their version, and reloads persisted data', async () => {
  const user = userEvent.setup();
  const { storage, props } = fixture();
  render(<PersistentNotes {...props} />);
  const editor = await screen.findByDisplayValue('Saved note');
  await user.clear(editor);
  await user.type(editor, 'Updated note');
  await user.click(screen.getByRole('button', { name: 'Save notes' }));
  await waitFor(() =>
    expect(storage.write).toHaveBeenCalledWith({
      text: 'Updated note',
      version: 3,
    })
  );
  expect(await screen.findByRole('status')).toHaveTextContent(
    'Saved to Vessel. Version 4.'
  );
  storage.read.mockResolvedValue({ text: 'Updated note', version: 4 });
  await user.click(screen.getByRole('button', { name: /Reload saved/ }));
  expect(await screen.findByDisplayValue('Updated note')).toBeVisible();
  expect(storage.read).toHaveBeenCalledTimes(2);
});

it('retains unsaved text when a conflicting write fails and allows explicit reload', async () => {
  const user = userEvent.setup();
  const { storage, props } = fixture();
  storage.write.mockRejectedValue(
    new Error('Notes changed; reload before saving')
  );
  render(<PersistentNotes {...props} />);
  const editor = await screen.findByDisplayValue('Saved note');
  await user.type(editor, ' local edit');
  await user.click(screen.getByRole('button', { name: 'Save notes' }));
  await waitFor(() =>
    expect(screen.getByRole('status')).toHaveTextContent('Notes changed')
  );
  expect(editor).toHaveValue('Saved note local edit');
});

it('blocks writes when the read grant is unavailable', async () => {
  const { storage, props } = fixture();
  storage.read.mockRejectedValue(new Error('Storage grant revoked'));
  render(<PersistentNotes {...props} />);
  await waitFor(() =>
    expect(screen.getByRole('status')).toHaveTextContent(
      'Storage grant revoked'
    )
  );
  expect(screen.getByRole('button', { name: 'Save notes' })).toBeDisabled();
  expect(storage.write).not.toHaveBeenCalled();
});
