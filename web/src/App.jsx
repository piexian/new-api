/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, {
  lazy,
  Suspense,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import {
  Route,
  Routes,
  useLocation,
  useNavigate,
  useParams,
} from 'react-router-dom';
import Loading from './components/common/ui/Loading';
import User from './pages/User';
import {
  API,
  AuthRedirect,
  PrivateRoute,
  AdminRoute,
  setUserData,
} from './helpers';
import RegisterForm from './components/auth/RegisterForm';
import LoginForm from './components/auth/LoginForm';
import NotFound from './pages/NotFound';
import Forbidden from './pages/Forbidden';
import Setting from './pages/Setting';
import { StatusContext } from './context/Status';
import { UserContext } from './context/User';
import { useTranslation } from 'react-i18next';
import { LOGIN_FEATURE_UPDATE_PROMPT_KEY } from './constants/common.constant';

import PasswordResetForm from './components/auth/PasswordResetForm';
import PasswordResetConfirm from './components/auth/PasswordResetConfirm';
import Channel from './pages/Channel';
import Token from './pages/Token';
import Redemption from './pages/Redemption';
import TopUp from './pages/TopUp';
import Log from './pages/Log';
import Chat from './pages/Chat';
import Chat2Link from './pages/Chat2Link';
import Midjourney from './pages/Midjourney';
import Pricing from './pages/Pricing';
import Task from './pages/Task';
import ModelPage from './pages/Model';
import ModelDeploymentPage from './pages/ModelDeployment';
import Playground from './pages/Playground';
import Subscription from './pages/Subscription';
import OAuth2Callback from './components/auth/OAuth2Callback';
import PersonalSetting from './components/settings/PersonalSetting';
import UpdateAnnouncementModal from './components/settings/personal/modals/UpdateAnnouncementModal';
import Setup from './pages/Setup';
import SetupCheck from './components/layout/SetupCheck';

const Home = lazy(() => import('./pages/Home'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const About = lazy(() => import('./pages/About'));
const UserAgreement = lazy(() => import('./pages/UserAgreement'));
const PrivacyPolicy = lazy(() => import('./pages/PrivacyPolicy'));

function DynamicOAuth2Callback() {
  const { provider } = useParams();
  return <OAuth2Callback type={provider} />;
}

function App() {
  const location = useLocation();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const [userState, userDispatch] = useContext(UserContext);
  const [showUpdateAnnouncement, setShowUpdateAnnouncement] = useState(false);
  const [announcementHasPassword, setAnnouncementHasPassword] = useState(true);

  // 获取模型广场权限配置
  const pricingRequireAuth = useMemo(() => {
    const headerNavModulesConfig = statusState?.status?.HeaderNavModules;
    if (headerNavModulesConfig) {
      try {
        const modules = JSON.parse(headerNavModulesConfig);

        // 处理向后兼容性：如果pricing是boolean，默认不需要登录
        if (typeof modules.pricing === 'boolean') {
          return false; // 默认不需要登录鉴权
        }

        // 如果是对象格式，使用requireAuth配置
        return modules.pricing?.requireAuth === true;
      } catch (error) {
        console.error('解析顶栏模块配置失败:', error);
        return false; // 默认不需要登录
      }
    }
    return false; // 默认不需要登录
  }, [statusState?.status?.HeaderNavModules]);

  useEffect(() => {
    if (
      !userState?.user?.id ||
      sessionStorage.getItem(LOGIN_FEATURE_UPDATE_PROMPT_KEY) !== '1'
    ) {
      return;
    }

    sessionStorage.removeItem(LOGIN_FEATURE_UPDATE_PROMPT_KEY);

    let cancelled = false;
    (async () => {
      try {
        const res = await API.get('/api/user/self');
        const { success, data } = res.data;
        if (!success || !data || cancelled) {
          return;
        }

        userDispatch({ type: 'login', payload: data });
        setUserData(data);

        if (
          data.setup_completed === 'pending' ||
          data.feature_update_v1 === 'dismissed'
        ) {
          return;
        }

        setAnnouncementHasPassword(data.has_password ?? true);
        setShowUpdateAnnouncement(true);
      } catch {
        // ignore and keep the current page flow
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [userDispatch, userState?.user?.id]);

  const handleDismissAnnouncement = async () => {
    setShowUpdateAnnouncement(false);
    try {
      await API.put('/api/user/self', { feature_update_v1: 'dismissed' });
      if (userState?.user) {
        const nextUser = {
          ...userState.user,
          feature_update_v1: 'dismissed',
        };
        userDispatch({ type: 'login', payload: nextUser });
        setUserData(nextUser);
      }
    } catch {
      // ignore
    }
  };

  return (
    <SetupCheck>
      <>
        <Routes>
          <Route
            path='/'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <Home />
              </Suspense>
            }
          />
          <Route
            path='/setup'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <Setup />
              </Suspense>
            }
          />
          <Route path='/forbidden' element={<Forbidden />} />
          <Route
            path='/console/models'
            element={
              <AdminRoute>
                <ModelPage />
              </AdminRoute>
            }
          />
          <Route
            path='/console/deployment'
            element={
              <AdminRoute>
                <ModelDeploymentPage />
              </AdminRoute>
            }
          />
          <Route
            path='/console/subscription'
            element={
              <AdminRoute>
                <Subscription />
              </AdminRoute>
            }
          />
          <Route
            path='/console/channel'
            element={
              <AdminRoute>
                <Channel />
              </AdminRoute>
            }
          />
          <Route
            path='/console/token'
            element={
              <PrivateRoute>
                <Token />
              </PrivateRoute>
            }
          />
          <Route
            path='/console/playground'
            element={
              <PrivateRoute>
                <Playground />
              </PrivateRoute>
            }
          />
          <Route
            path='/console/redemption'
            element={
              <AdminRoute>
                <Redemption />
              </AdminRoute>
            }
          />
          <Route
            path='/console/user'
            element={
              <AdminRoute>
                <User />
              </AdminRoute>
            }
          />
          <Route
            path='/user/reset'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <PasswordResetConfirm />
              </Suspense>
            }
          />
          <Route
            path='/login'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <AuthRedirect>
                  <LoginForm />
                </AuthRedirect>
              </Suspense>
            }
          />
          <Route
            path='/register'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <AuthRedirect>
                  <RegisterForm />
                </AuthRedirect>
              </Suspense>
            }
          />
          <Route
            path='/reset'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <PasswordResetForm />
              </Suspense>
            }
          />
          <Route
            path='/oauth/github'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <OAuth2Callback type='github'></OAuth2Callback>
              </Suspense>
            }
          />
          <Route
            path='/oauth/discord'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <OAuth2Callback type='discord'></OAuth2Callback>
              </Suspense>
            }
          />
          <Route
            path='/oauth/oidc'
            element={
              <Suspense fallback={<Loading></Loading>}>
                <OAuth2Callback type='oidc'></OAuth2Callback>
              </Suspense>
            }
          />
          <Route
            path='/oauth/linuxdo'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <OAuth2Callback type='linuxdo'></OAuth2Callback>
              </Suspense>
            }
          />
          <Route
            path='/oauth/:provider'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <DynamicOAuth2Callback />
              </Suspense>
            }
          />
          <Route
            path='/console/setting'
            element={
              <AdminRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Setting />
                </Suspense>
              </AdminRoute>
            }
          />
          <Route
            path='/console/personal'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <PersonalSetting />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route
            path='/console/topup'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <TopUp />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route
            path='/console/log'
            element={
              <PrivateRoute>
                <Log />
              </PrivateRoute>
            }
          />
          <Route
            path='/console'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Dashboard />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route
            path='/console/midjourney'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Midjourney />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route
            path='/console/task'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Task />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route
            path='/pricing'
            element={
              pricingRequireAuth ? (
                <PrivateRoute>
                  <Suspense
                    fallback={<Loading></Loading>}
                    key={location.pathname}
                  >
                    <Pricing />
                  </Suspense>
                </PrivateRoute>
              ) : (
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Pricing />
                </Suspense>
              )
            }
          />
          <Route
            path='/about'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <About />
              </Suspense>
            }
          />
          <Route
            path='/user-agreement'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <UserAgreement />
              </Suspense>
            }
          />
          <Route
            path='/privacy-policy'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <PrivacyPolicy />
              </Suspense>
            }
          />
          <Route
            path='/console/chat/:id?'
            element={
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <Chat />
              </Suspense>
            }
          />
          {/* 方便使用chat2link直接跳转聊天... */}
          <Route
            path='/chat2link'
            element={
              <PrivateRoute>
                <Suspense
                  fallback={<Loading></Loading>}
                  key={location.pathname}
                >
                  <Chat2Link />
                </Suspense>
              </PrivateRoute>
            }
          />
          <Route path='*' element={<NotFound />} />
        </Routes>
        <UpdateAnnouncementModal
          t={t}
          visible={showUpdateAnnouncement}
          onClose={handleDismissAnnouncement}
          hasPassword={announcementHasPassword}
          onChangePassword={() =>
            navigate('/console/personal', {
              state: { openChangePasswordModal: true },
            })
          }
        />
      </>
    </SetupCheck>
  );
}

export default App;
