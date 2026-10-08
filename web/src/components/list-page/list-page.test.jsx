// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { BatchBar, ListEmptyState, ListFooter, ListPage, ListPageHeader, ListToolbar } from './index';

// 统一列表页模板:页头统计 chip、工具栏搜索 / 分面 pill、批量条显隐、两种空态、分页边界,以及四段组合结构。

afterEach(cleanup);

const t = i18n.t.bind(i18n);

const FIELDS = [
  {
    key: 'status',
    type: 'enum',
    labelKey: 'Status',
    paramInclude: 'status',
    options: [
      { value: '1', label: 'Enabled' },
      { value: '2', label: 'Disabled' }
    ]
  },
  { key: 'group', type: 'text', labelKey: 'Group' }
];

describe('ListPageHeader', () => {
  it('renders title, description, actions and stat chips', () => {
    render(
      <ListPageHeader
        title="Models"
        description="All models"
        actions={<button type="button">New</button>}
        stats={[
          { id: 'all', label: 'All', count: 12, active: true, onClick: () => {} },
          { id: 'off', label: 'Hidden', count: 3, tone: 'warning' }
        ]}
      />
    );
    expect(screen.getByRole('heading', { name: 'Models' })).toBeTruthy();
    expect(screen.getByText('All models')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'New' })).toBeTruthy();
    expect(screen.getByRole('button', { name: /All/ }).getAttribute('aria-pressed')).toBe('true');
    expect(screen.queryByRole('button', { name: /Hidden/ })).toBeNull();
    expect(screen.getByText('Hidden')).toBeTruthy();
  });

  it('calls the stat chip onClick', () => {
    const onClick = vi.fn();
    render(<ListPageHeader title="Users" stats={[{ id: 'banned', label: 'Banned', count: 2, onClick }]} />);
    const chip = screen.getByRole('button', { name: /Banned/ });
    expect(chip.getAttribute('aria-pressed')).toBe('false');
    fireEvent.click(chip);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});

describe('ListToolbar', () => {
  it('drives the controlled search box and clears it', () => {
    const onChange = vi.fn();
    const { rerender } = render(<ListToolbar search={{ value: '', onChange }} />);
    fireEvent.change(screen.getByRole('searchbox'), { target: { value: 'gpt' } });
    expect(onChange).toHaveBeenLastCalledWith('gpt');
    expect(screen.queryByRole('button', { name: t('listPage.clearSearch') })).toBeNull();
    rerender(<ListToolbar search={{ value: 'gpt', onChange }} />);
    fireEvent.click(screen.getByRole('button', { name: t('listPage.clearSearch') }));
    expect(onChange).toHaveBeenLastCalledWith('');
  });

  it('clears one filter pill at a time and renders the view controls slot', () => {
    const state = { status: { op: 'in', values: ['1'] }, group: { value: 'vip' } };
    const onChange = vi.fn();
    render(<ListToolbar filters={{ fields: FIELDS, state, onChange }} viewControls={<button type="button">Columns</button>} />);
    expect(screen.getByText('Enabled')).toBeTruthy();
    expect(screen.getByText('vip')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Columns' })).toBeTruthy();
    const removers = screen.getAllByRole('button', { name: t('filterBar.remove') });
    expect(removers).toHaveLength(2);
    fireEvent.click(removers[0]);
    const updater = onChange.mock.calls[0][0];
    expect(updater(state)).toEqual({ group: { value: 'vip' } });
  });
});

describe('BatchBar', () => {
  it('renders nothing without a selection', () => {
    const { container } = render(<BatchBar count={0} onClear={() => {}} />);
    expect(container.innerHTML).toBe('');
  });

  it('shows the count, actions and a clear button', () => {
    const onClear = vi.fn();
    render(<BatchBar count={3} onClear={onClear} actions={<button type="button">Enable</button>} />);
    expect(screen.getByRole('toolbar')).toBeTruthy();
    expect(screen.getAllByText(t('listPage.selected', { count: 3 })).length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Enable' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: t('listPage.clearSelection') }));
    expect(onClear).toHaveBeenCalledTimes(1);
  });
});

describe('ListEmptyState', () => {
  it('no-data variant shows the primary action', () => {
    render(<ListEmptyState action={<button type="button">Add model</button>} />);
    expect(screen.getByText(t('listPage.emptyTitle'))).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Add model' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: t('listPage.clearFilters') })).toBeNull();
  });

  it('no-match variant offers clear filters', () => {
    const onClearFilters = vi.fn();
    render(<ListEmptyState variant="noMatch" onClearFilters={onClearFilters} />);
    expect(screen.getByText(t('listPage.noMatchTitle'))).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: t('listPage.clearFilters') }));
    expect(onClearFilters).toHaveBeenCalledTimes(1);
  });
});

describe('ListPage composition', () => {
  const compose = (selectedCount) =>
    render(
      <ListPage
        header={<ListPageHeader title="Channels" />}
        toolbar={<ListToolbar search={{ value: '', onChange: () => {} }} />}
        batchBar={<BatchBar count={selectedCount} onClear={() => {}} />}
        selectedCount={selectedCount}
        footer={<ListFooter total={5} page={0} pageSize={10} onPageChange={() => {}} />}
      >
        <table />
      </ListPage>
    );

  it('renders header, toolbar, body and footer in order', () => {
    const { container } = compose(0);
    const slots = [...container.querySelector('[data-slot="list-page"]').children].map((el) => el.dataset.slot);
    expect(slots).toEqual(['list-page-header', 'list-page-toolbar', 'list-page-body', 'list-page-footer']);
    expect(screen.getByRole('searchbox')).toBeTruthy();
    expect(screen.queryByRole('toolbar')).toBeNull();
  });

  it('swaps the toolbar for the batch bar when rows are selected', () => {
    const { container } = compose(2);
    const toolbarSlot = container.querySelector('[data-slot="list-page-toolbar"]');
    expect(within(toolbarSlot).getByRole('toolbar')).toBeTruthy();
    expect(screen.queryByRole('searchbox')).toBeNull();
  });
});

describe('ListFooter', () => {
  const setup = (props) => {
    const onPageChange = vi.fn();
    const onPageSizeChange = vi.fn();
    render(<ListFooter total={45} page={0} pageSize={10} onPageChange={onPageChange} onPageSizeChange={onPageSizeChange} {...props} />);
    const btn = (key) => screen.getByRole('button', { name: t(`listPage.${key}`) });
    return { onPageChange, onPageSizeChange, btn };
  };

  it('disables first / prev on the first page', () => {
    const { btn, onPageChange } = setup();
    expect(screen.getByText(t('listPage.pageOf', { page: 1, pages: 5 }))).toBeTruthy();
    expect(screen.getByText(t('pagination.total', { count: 45 }))).toBeTruthy();
    expect(btn('firstPage').disabled).toBe(true);
    expect(btn('prevPage').disabled).toBe(true);
    fireEvent.click(btn('nextPage'));
    expect(onPageChange).toHaveBeenLastCalledWith(1);
    fireEvent.click(btn('lastPage'));
    expect(onPageChange).toHaveBeenLastCalledWith(4);
  });

  it('disables next / last on the last page', () => {
    const { btn, onPageChange } = setup({ page: 4 });
    expect(btn('nextPage').disabled).toBe(true);
    expect(btn('lastPage').disabled).toBe(true);
    fireEvent.click(btn('prevPage'));
    expect(onPageChange).toHaveBeenLastCalledWith(3);
  });

  it('treats an empty list as a single page and changes page size', () => {
    const { btn, onPageSizeChange } = setup({ total: 0, selectedCount: 2 });
    expect(screen.getByText(t('listPage.pageOf', { page: 1, pages: 1 }))).toBeTruthy();
    expect(btn('prevPage').disabled).toBe(true);
    expect(btn('nextPage').disabled).toBe(true);
    expect(screen.getByText(new RegExp(t('listPage.selectedOf', { count: 2 })))).toBeTruthy();
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '50' } });
    expect(onPageSizeChange).toHaveBeenCalledWith(50);
  });
});
