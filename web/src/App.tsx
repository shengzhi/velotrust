import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, ProtectedRoute } from './contexts/AuthContext';
import { ToastProvider } from './components/ui/toast';
import { AppLayout } from './components/Layout/AppLayout';

import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { NodeList } from './pages/Nodes/NodeList';
import { NodeDetail } from './pages/Nodes/NodeDetail';
import { Topology } from './pages/Topology';
import { UserList } from './pages/Users/UserList';
import { EmployeeList } from './pages/Users/EmployeeList';
import { GroupList } from './pages/Users/GroupList';
import { Devices } from './pages/Devices';
import { Sites } from './pages/Sites';
import { Policies } from './pages/Policies';
import { Audit } from './pages/Audit';
import { Dns } from './pages/Dns';
import { Idp } from './pages/Idp';
import { AccessMatrix } from './pages/AccessMatrix';
import { Ipam } from './pages/Ipam';
import { Settings } from './pages/Settings';
import { Resources } from './pages/Resources';
import { Roles } from './pages/Roles';
import { Menus } from './pages/Menus';
import { Permissions } from './pages/Permissions';
import { LoginLogs } from './pages/LoginLogs';
import { OperationLogs } from './pages/OperationLogs';
import { EnrollmentTokens } from './pages/EnrollmentTokens';
import { Profile } from './pages/Profile';

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider>
          <Routes>
            {/* Unauthenticated route */}
            <Route path="/login" element={<Login />} />

            {/* Authenticated route tree inside AppLayout */}
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <AppLayout />
                </ProtectedRoute>
              }
            >
              <Route index element={<Dashboard />} />
              <Route path="nodes" element={<NodeList />} />
              <Route path="nodes/:id" element={<NodeDetail />} />
              <Route path="enrollment-tokens" element={<EnrollmentTokens />} />
              <Route path="topology" element={<Topology />} />
              <Route path="resources" element={<Resources />} />
              <Route path="users" element={<UserList />} />
              <Route path="employee-users" element={<EmployeeList />} />
              <Route path="groups" element={<GroupList />} />
              <Route path="roles" element={<Roles />} />
              <Route path="menus" element={<Menus />} />
              <Route path="permissions" element={<Permissions />} />
              <Route path="api-mappings" element={<Permissions />} />
              <Route path="devices" element={<Devices />} />
              <Route path="sites" element={<Sites />} />
              <Route path="ipam" element={<Ipam />} />
              <Route path="dns" element={<Dns />} />
              <Route path="policies" element={<Policies />} />
              <Route path="access-matrix" element={<AccessMatrix />} />
              <Route path="idp" element={<Idp />} />
              <Route path="audit" element={<Audit />} />
              <Route path="operation-logs" element={<OperationLogs />} />
              <Route path="login-logs" element={<LoginLogs />} />
              <Route path="settings" element={<Settings />} />
              <Route path="profile" element={<Profile />} />
            </Route>


            {/* Catch-all fallback */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
