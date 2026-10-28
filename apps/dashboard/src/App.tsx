import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { TelemetryProvider } from './context/TelemetryContext';
import { Layout } from './components/layout/Layout';
import { DashboardOverview } from './pages/DashboardOverview';
import { TracesPage } from './pages/TracesPage';
import { LogsPage } from './pages/LogsPage';
import { TopologyPage } from './pages/TopologyPage';
import { AnomaliesPage } from './pages/AnomaliesPage';
import { AssistantPage } from './pages/AssistantPage';
import { SettingsPage } from './pages/SettingsPage';
import './styles/dashboard.css';

export function App() {
  return (
    <TelemetryProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Layout />}>
            <Route index element={<DashboardOverview />} />
            <Route path="traces" element={<TracesPage />} />
            <Route path="logs" element={<LogsPage />} />
            <Route path="topology" element={<TopologyPage />} />
            <Route path="anomalies" element={<AnomaliesPage />} />
            <Route path="assistant" element={<AssistantPage />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="login" element={<Navigate to="/" replace />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </TelemetryProvider>
  );
}

export default App;
