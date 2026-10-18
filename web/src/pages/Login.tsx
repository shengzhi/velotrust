import React, { useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { ArrowRight, Fingerprint, Layers, Lock, ShieldCheck, User as UserIcon } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';

export const Login: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const { login, loginWithPasskey } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/';

  const handlePasswordLogin = async (event: React.FormEvent) => {
    event.preventDefault();
    setError('');
    setLoading(true);
    try {
      await login(username.trim(), password, totpCode);
      navigate(from, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败，请重试。');
    } finally {
      setLoading(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setError('');
    setLoading(true);
    try {
      await loginWithPasskey();
      navigate(from, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : '通行密钥登录失败。');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-slate-900 p-4">
      <div className="absolute inset-0 bg-[radial-gradient(#3b82f6_1px,transparent_1px)] [background-size:24px_24px] opacity-15" />
      <div className="absolute -left-20 top-1/4 h-96 w-96 rounded-full bg-blue-600/20 blur-3xl" />
      <div className="absolute -right-20 bottom-1/4 h-96 w-96 rounded-full bg-indigo-600/20 blur-3xl" />

      <div className="relative z-10 w-full max-w-md rounded-2xl border border-slate-100 bg-white p-8 shadow-2xl sm:p-10">
        <div className="mb-8 flex flex-col items-center text-center">
          <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-tr from-blue-700 to-indigo-600 text-white shadow-lg shadow-blue-500/30">
            <Layers className="h-8 w-8" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900">VeloTrust 控制台</h1>
          <p className="mt-1 text-sm text-slate-500">使用 Management IAM 账号登录</p>
        </div>

        {error && (
          <div className="mb-5 flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700" role="alert">
            <span className="font-semibold">登录失败：</span>
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handlePasswordLogin} className="space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-700">用户名</label>
            <Input
              type="text"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="输入用户名"
              leftIcon={<UserIcon className="h-4 w-4" />}
              autoComplete="username"
              required
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-700">密码</label>
            <Input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="输入密码"
              leftIcon={<Lock className="h-4 w-4" />}
              autoComplete="current-password"
              required
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-slate-700">TOTP 验证码（如已启用）</label>
            <Input
              type="text"
              inputMode="numeric"
              value={totpCode}
              onChange={(event) => setTotpCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="6 位验证码"
              autoComplete="one-time-code"
              maxLength={6}
            />
          </div>
          <Button type="submit" className="h-11 w-full gap-2 text-sm font-semibold" isLoading={loading}>
            <span>登录</span>
            <ArrowRight className="h-4 w-4" />
          </Button>
        </form>

        <div className="my-5 flex items-center gap-3 text-[10px] text-slate-400">
          <span className="h-px flex-1 bg-slate-200" />
          <span>或</span>
          <span className="h-px flex-1 bg-slate-200" />
        </div>

        <Button
          type="button"
          variant="outline"
          className="h-11 w-full gap-2"
          isLoading={loading}
          onClick={() => void handlePasskeyLogin()}
        >
          <Fingerprint className="h-4 w-4" />
          <span>使用通行密钥登录</span>
        </Button>

        <div className="mt-6 flex items-start gap-2 border-t border-slate-100 pt-5 text-[11px] leading-relaxed text-slate-500">
          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
          <span>登录凭据由 Management 校验。登录令牌只保存在当前浏览器标签页会话中。</span>
        </div>
      </div>
    </div>
  );
};
