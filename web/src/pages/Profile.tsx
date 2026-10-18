import React, { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Fingerprint, KeyRound, LockKeyhole, ShieldCheck, UserRound } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { iamApi, IAMPasskey } from '../services/iam';
import { createPasskey } from '../services/webauthn';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Badge } from '../components/ui/badge';
import { Card } from '../components/ui/card';
import { QrCodeSvg } from '../components/ui/QrCodeSvg';
import { useToast } from '../components/ui/toast';
import { formatDateTime } from '../utils/format';

type ProfileTab = 'profile' | 'password' | 'totp' | 'passkey';
type TOTPSetup = { secret: string; otpauth_uri: string };

const tabs: { id: ProfileTab; label: string; icon: LucideIcon }[] = [
  { id: 'profile', label: '账号信息', icon: UserRound },
  { id: 'password', label: '修改密码', icon: LockKeyhole },
  { id: 'totp', label: '两步验证', icon: ShieldCheck },
  { id: 'passkey', label: '通行密钥', icon: Fingerprint },
];

export const Profile: React.FC = () => {
  const { user, refreshCurrentUser } = useAuth();
  const { success, error } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const initialTab = searchParams.get('tab');
  const [tab, setTab] = useState<ProfileTab>(['profile', 'password', 'totp', 'passkey'].includes(initialTab ?? '') ? initialTab as ProfileTab : 'profile');
  const [busy, setBusy] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const [totpPassword, setTotpPassword] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpSetup, setTotpSetup] = useState<TOTPSetup | null>(null);

  const [passkeyName, setPasskeyName] = useState('');
  const [passkeys, setPasskeys] = useState<IAMPasskey[]>(user?.passkeys ?? []);

  useEffect(() => {
    if (initialTab && ['profile', 'password', 'totp', 'passkey'].includes(initialTab)) setTab(initialTab as ProfileTab);
  }, [initialTab]);

  useEffect(() => {
    if (user?.must_change_password) {
      setTab('password');
      setSearchParams({ tab: 'password' }, { replace: true });
    }
  }, [user?.must_change_password, setSearchParams]);

  useEffect(() => setPasskeys(user?.passkeys ?? []), [user?.passkeys]);

  const selectTab = (next: ProfileTab) => {
    setTab(next);
    setSearchParams(next === 'profile' ? {} : { tab: next });
  };

  const changePassword = async (event: React.FormEvent) => {
    event.preventDefault();
    if (Array.from(newPassword).length < 8) return error('新密码至少需要 8 个字符。');
    if (newPassword !== confirmPassword) return error('两次输入的新密码不一致。');
    if (newPassword === currentPassword) return error('新密码不能与当前密码相同。');
    setBusy(true);
    try {
      await iamApi.changePassword(currentPassword, newPassword);
      await refreshCurrentUser();
      setCurrentPassword(''); setNewPassword(''); setConfirmPassword('');
      success('密码已修改。当前会话保留，其他会话已由服务端撤销。');
    } catch (err) {
      error(err instanceof Error ? err.message : '修改密码失败。');
    } finally {
      setBusy(false);
    }
  };

  const beginTOTP = async (event: React.FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      const setup = await iamApi.setupTOTP(totpPassword);
      setTotpSetup(setup);
      setTotpPassword('');
      setTotpCode('');
    } catch (err) {
      error(err instanceof Error ? err.message : '无法开始 TOTP 配置。');
    } finally {
      setBusy(false);
    }
  };

  const confirmTOTP = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!/^\d{6}$/.test(totpCode)) return error('请输入 6 位验证码。');
    setBusy(true);
    try {
      await iamApi.confirmTOTP(totpCode);
      await refreshCurrentUser();
      setTotpSetup(null); setTotpCode('');
      success('两步验证已启用。请确认保留可用的认证器设备；当前版本未提供恢复码。');
    } catch (err) {
      error(err instanceof Error ? err.message : '验证 TOTP 失败。');
    } finally {
      setBusy(false);
    }
  };

  const disableTOTP = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!/^\d{6}$/.test(totpCode)) return error('请输入 6 位验证码。');
    setBusy(true);
    try {
      await iamApi.disableTOTP(totpPassword, totpCode);
      await refreshCurrentUser();
      setTotpPassword(''); setTotpCode('');
      success('两步验证已关闭。');
    } catch (err) {
      error(err instanceof Error ? err.message : '关闭 TOTP 失败。');
    } finally {
      setBusy(false);
    }
  };

  const registerPasskey = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!passkeyName.trim()) return error('请填写通行密钥名称。');
    setBusy(true);
    try {
      const { ceremony } = await iamApi.beginPasskeyRegistration();
      if (!ceremony) throw new Error('Management 未返回通行密钥注册挑战。');
      const response = await createPasskey(ceremony.options);
      await iamApi.finishPasskeyRegistration(ceremony.challenge_id, passkeyName.trim(), response);
      await refreshCurrentUser();
      setPasskeyName('');
      success('通行密钥已注册。');
    } catch (err) {
      error(err instanceof Error ? err.message : '注册通行密钥失败。');
    } finally {
      setBusy(false);
    }
  };

  const revokePasskey = async (passkey: IAMPasskey) => {
    if (!window.confirm(`确定撤销通行密钥「${passkey.name}」吗？`)) return;
    setBusy(true);
    try {
      await iamApi.revokePasskey(passkey.id);
      await refreshCurrentUser();
      success('通行密钥已撤销。');
    } catch (err) {
      error(err instanceof Error ? err.message : '撤销通行密钥失败。');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-slate-900">个人信息与安全</h1>
          <p className="mt-0.5 text-xs text-slate-500">账号资料、密码及登录凭证</p>
        </div>
      </div>

      <Card className="overflow-x-auto p-0">
        <nav className="flex min-w-max items-center gap-1 px-3" aria-label="个人信息设置">
          {tabs.filter(({ id }) => !user?.must_change_password || id === 'profile' || id === 'password').map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              type="button"
              onClick={() => selectTab(id)}
              className={`flex items-center gap-2 border-b-2 px-4 py-3 text-xs font-semibold transition-colors ${tab === id ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:border-slate-300 hover:text-slate-800'}`}
              aria-current={tab === id ? 'page' : undefined}
            >
              <Icon className="h-4 w-4" />{label}
            </button>
          ))}
        </nav>
      </Card>

      <div className="space-y-5">
          {tab === 'profile' && (
            <Card className="p-6">
              <div className="mb-5 flex items-center gap-3"><div className="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-50 text-blue-700"><UserRound className="h-5 w-5" /></div><div><h2 className="font-semibold text-slate-900">Management IAM 账号</h2><p className="text-xs text-slate-500">身份资料由 IAM 用户目录提供</p></div></div>
              <dl className="grid gap-4 sm:grid-cols-2">{[
                ['显示名称', user?.display_name || '—'], ['用户名', user?.username || '—'], ['邮箱', user?.email || '—'], ['状态', user?.user_status || '—'], ['创建时间', user?.created_at ? formatDateTime(user.created_at) : '—'], ['最近登录', user?.last_login_at ? formatDateTime(user.last_login_at) : '—'],
              ].map(([label, value]) => <div key={label} className="rounded-lg border border-slate-100 p-3"><dt className="text-[11px] text-slate-500">{label}</dt><dd className="mt-1 break-words text-sm font-medium text-slate-800">{value}</dd></div>)}</dl>
              {user?.must_change_password && <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">管理员设置的临时密码必须修改后才能使用其他 IAM 功能。</div>}
            </Card>
          )}

          {tab === 'password' && (
            <Card className="p-6">
              <h2 className="font-semibold text-slate-900">修改登录密码</h2><p className="mt-1 text-xs text-slate-500">新密码至少 8 个字符。修改后服务端保留当前会话并撤销其他会话。</p>
              <form className="mt-5 max-w-lg space-y-4" onSubmit={changePassword}>
                <label className="block space-y-1.5 text-xs font-medium text-slate-700">当前密码<Input type="password" autoComplete="current-password" required maxLength={1024} value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} /></label>
                <label className="block space-y-1.5 text-xs font-medium text-slate-700">新密码<Input type="password" autoComplete="new-password" required minLength={8} maxLength={1024} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} /></label>
                <label className="block space-y-1.5 text-xs font-medium text-slate-700">确认新密码<Input type="password" autoComplete="new-password" required minLength={8} maxLength={1024} value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} /></label>
                <Button type="submit" isLoading={busy}><LockKeyhole className="h-4 w-4" />更新密码</Button>
              </form>
            </Card>
          )}

          {tab === 'totp' && (
            <Card className="p-6">
      <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-semibold text-slate-900">基于时间的一次性密码 (TOTP)</h2><p className="mt-1 text-xs text-slate-500">启用后，密码登录还需要认证器生成的 6 位验证码。</p></div><Badge variant={user?.totp_enabled ? 'success' : 'warning'}>{user?.totp_enabled ? '已启用' : '未启用'}</Badge></div>
              {!user?.totp_enabled && !totpSetup && (
                <form className="mt-5 max-w-lg space-y-4" onSubmit={beginTOTP}><label className="block space-y-1.5 text-xs font-medium text-slate-700">重新输入当前密码<Input type="password" autoComplete="current-password" required value={totpPassword} onChange={(event) => setTotpPassword(event.target.value)} /></label><Button type="submit" isLoading={busy}>开始绑定认证器</Button></form>
              )}
              {!user?.totp_enabled && totpSetup && (
                <form className="mt-5 max-w-2xl space-y-4" onSubmit={confirmTOTP}>
                  <div className="grid gap-5 sm:grid-cols-[240px_minmax(0,1fr)]">
                    <div className="flex flex-col items-center gap-3 rounded-lg border border-slate-200 bg-white p-4">
                      <QrCodeSvg value={totpSetup.otpauth_uri} size={208} />
                      <p className="text-center text-xs text-slate-500">使用认证器应用扫描二维码</p>
                    </div>
                    <div className="space-y-4">
                      <div className="rounded-lg border border-blue-100 bg-blue-50 p-4 text-sm leading-6 text-blue-900">二维码在本机浏览器中生成。请勿将二维码或设置密钥分享给他人；当前 IAM 未提供恢复码，绑定前请确认认证器可正常使用。</div>
                      <div><div className="mb-1 text-xs font-medium text-slate-600">无法扫码？手动输入设置密钥</div><code className="block break-all rounded-lg bg-slate-900 px-4 py-3 font-mono text-sm tracking-wider text-white">{totpSetup.secret}</code></div>
                      <details className="rounded-lg border border-slate-200 p-3"><summary className="cursor-pointer text-xs font-medium text-slate-700">查看 otpauth URI</summary><code className="mt-2 block break-all text-xs text-slate-600">{totpSetup.otpauth_uri}</code></details>
                    </div>
                  </div>
                  <label className="block max-w-xs space-y-1.5 text-xs font-medium text-slate-700">认证器当前显示的 6 位验证码<Input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required value={totpCode} onChange={(event) => setTotpCode(event.target.value.replace(/\D/g, '').slice(0, 6))} /></label>
                  <div className="flex gap-2"><Button type="submit" isLoading={busy}>验证并启用</Button><Button type="button" variant="outline" disabled={busy} onClick={() => { setTotpSetup(null); setTotpCode(''); }}>取消</Button></div>
                </form>
              )}
              {user?.totp_enabled && <form className="mt-5 max-w-lg space-y-4" onSubmit={disableTOTP}><div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-800">关闭两步验证需要重新验证当前密码和认证器验证码。</div><label className="block space-y-1.5 text-xs font-medium text-slate-700">当前密码<Input type="password" autoComplete="current-password" required value={totpPassword} onChange={(event) => setTotpPassword(event.target.value)} /></label><label className="block space-y-1.5 text-xs font-medium text-slate-700">TOTP 验证码<Input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required value={totpCode} onChange={(event) => setTotpCode(event.target.value.replace(/\D/g, '').slice(0, 6))} /></label><Button type="submit" variant="danger" isLoading={busy}>关闭两步验证</Button></form>}
            </Card>
          )}

          {tab === 'passkey' && (
            <Card className="p-6">
              <div className="flex items-center gap-2"><Fingerprint className="h-5 w-5 text-blue-600" /><h2 className="font-semibold text-slate-900">通行密钥 (Passkey)</h2></div>
              <p className="mt-1 text-xs text-slate-500">使用浏览器 WebAuthn API 注册。需要在 HTTPS 或 localhost 的安全上下文中访问。</p>
              <form className="mt-5 flex max-w-xl flex-col gap-3 sm:flex-row" onSubmit={registerPasskey}><Input required maxLength={128} placeholder="为这台设备命名，例如：办公笔记本" value={passkeyName} onChange={(event) => setPasskeyName(event.target.value)} /><Button type="submit" isLoading={busy} className="shrink-0"><KeyRound className="h-4 w-4" />注册通行密钥</Button></form>
              <div className="mt-6 divide-y divide-slate-100 rounded-lg border border-slate-100">
                {passkeys.length === 0 ? <div className="p-6 text-center text-sm text-slate-500">此账号尚未注册通行密钥。</div> : passkeys.map((passkey) => <div key={passkey.id} className="flex flex-col justify-between gap-3 p-4 sm:flex-row sm:items-center"><div className="flex items-start gap-3"><div className="rounded-lg bg-slate-100 p-2 text-slate-600"><Fingerprint className="h-4 w-4" /></div><div><div className="text-sm font-medium text-slate-800">{passkey.name || '未命名通行密钥'}</div><div className="mt-1 text-xs text-slate-500">注册于 {formatDateTime(passkey.created_at)} · 最近使用 {passkey.last_used_at ? formatDateTime(passkey.last_used_at) : '尚未使用'}</div></div></div><Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void revokePasskey(passkey)}>撤销</Button></div>)}</div>
            </Card>
          )}
      </div>
    </div>
  );
};
