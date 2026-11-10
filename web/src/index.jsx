import { createRoot } from 'react-dom/client';

// third party
import { RouterProvider, createBrowserRouter } from 'react-router';
import { Provider } from 'react-redux';

// project imports
import App from 'App';
import { store } from 'store';

// style + assets
import 'assets/scss/style.scss';
// Tailwind v4 + shadcn tokens — imported after MUI styles so utilities win cascade
import './styles/tailwind.css';
import config from './config';
import RouteError from './components/RouteError';
// ==============================|| REACT DOM RENDER  ||============================== //

// A data router with one catch-all route: the app keeps declaring its routes with useRoutes,
// and pages can use data-router APIs such as useBlocker (the settings leave guard).
const router = createBrowserRouter([{ path: '*', element: <App />, errorElement: <RouteError /> }], { basename: config.basename });

const container = document.getElementById('root');
const root = createRoot(container); // createRoot(container!) if you use TypeScript
root.render(
  <Provider store={store}>
    <RouterProvider router={router} />
  </Provider>
);
