import { createRoot } from 'react-dom/client';

// third party
import { BrowserRouter } from 'react-router';
import { Provider } from 'react-redux';

// project imports
import App from 'App';
import { store } from 'store';

// style + assets
import 'assets/scss/style.scss';
// Tailwind v4 + shadcn tokens — imported after MUI styles so utilities win cascade
import './styles/tailwind.css';
import config from './config';
// ==============================|| REACT DOM RENDER  ||============================== //

const container = document.getElementById('root');
const root = createRoot(container); // createRoot(container!) if you use TypeScript
root.render(
  <Provider store={store}>
    <BrowserRouter basename={config.basename}>
      <App />
    </BrowserRouter>
  </Provider>
);
