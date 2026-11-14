import { createContext, useContext } from 'react';

// ==============================|| CHROME — SEARCH ||============================== //
// There is one search in the panel: the command palette (CommandMenu, ⌘K). Every search button
// (the sidebars' top row, the platform settings subnav) opens it. A page can add its own entries
// to it: a settings page registers a function that lists its cards with their text, read when the
// palette opens, so the palette finds a card by anything written on it ("On this page").
//
// value: { openSearch(), registerPageIndex(fn) → unregister }, or null outside MainLayout.

export const SearchContext = createContext(null);

export const useSearch = () => useContext(SearchContext);
