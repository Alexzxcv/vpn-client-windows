import { observer } from 'mobx-react-lite';
import { AppRouter } from '@/router/AppRouter';
import { useAuth } from '@/stores/context';

export const App = observer(function App() {
  const auth = useAuth();

  if (auth.bootstrapError) {
    return (
      <div className="app">
        <main className="app-main">
          <div className="error">
            Не удалось связаться с ядром: {auth.bootstrapError}
          </div>
        </main>
      </div>
    );
  }

  if (!auth.bootstrapped) {
    return (
      <div className="app">
        <main className="app-main">
          <div className="loading">Инициализация…</div>
        </main>
      </div>
    );
  }

  return <AppRouter />;
});
