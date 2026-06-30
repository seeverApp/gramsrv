import type { FormEvent } from "react";
import { useState } from "react";
import { api, errorMessage } from "../api";
import { Alert } from "../components/ui";

export function LoginPage({ onLogin }: { onLogin: (actor: string) => void }) {
  const [secret, setSecret] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await api.login(secret);
      onLogin(result.actor);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-panel">
        <div className="login-head">
          <div className="brand brand-elevated">
            <span className="brand-mark">T</span>
            <span>
              <strong>telesrv</strong>
              <small>管理控制台</small>
            </span>
          </div>
          <span className="login-chip">本地访问</span>
        </div>
        <div className="login-copy">
          <h1>运维后台</h1>
          <p>输入凭据后进入控制台。</p>
        </div>
        {error && <Alert>{error}</Alert>}
        <form className="form-stack" onSubmit={submit}>
          <label>
            <span>管理员密码或 token</span>
            <input
              autoFocus
              type="password"
              value={secret}
              autoComplete="current-password"
              onChange={(event) => setSecret(event.target.value)}
            />
          </label>
          <button className="btn primary full" type="submit" disabled={busy}>
            {busy ? "登录中" : "登录"}
          </button>
        </form>
      </section>
    </main>
  );
}
