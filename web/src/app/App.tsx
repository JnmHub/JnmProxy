import { Navigate, Route, Routes } from 'react-router-dom';
import { AppShell } from '../components/layout/AppShell';
import { CredentialsPage } from '../pages/CredentialsPage';
import { DashboardPage } from '../pages/DashboardPage';
import { GroupsPage } from '../pages/GroupsPage';
import { KeywordGroupsPage } from '../pages/KeywordGroupsPage';
import { NodesPage } from '../pages/NodesPage';
import { OperationLogsPage } from '../pages/OperationLogsPage';
import { SettingsPage } from '../pages/SettingsPage';
import { StatsPage } from '../pages/StatsPage';
import { SubscriptionDetailPage } from '../pages/SubscriptionDetailPage';
import { SubscriptionsPage } from '../pages/SubscriptionsPage';
import { SystemPage } from '../pages/SystemPage';

export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="subscriptions" element={<SubscriptionsPage />} />
        <Route path="subscriptions/:id" element={<SubscriptionDetailPage />} />
        <Route path="nodes" element={<NodesPage />} />
        <Route path="groups" element={<GroupsPage />} />
        <Route path="keyword-groups" element={<KeywordGroupsPage />} />
        <Route path="credentials" element={<CredentialsPage />} />
        <Route path="stats" element={<StatsPage />} />
        <Route path="operation-logs" element={<OperationLogsPage />} />
        <Route path="system" element={<SystemPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Route>
    </Routes>
  );
}
