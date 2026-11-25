import { useState } from "react";
import { ArrowRight, Lock, Ticket, UserRound } from "lucide-react";
import { publishedImageURL } from "./published-image";
import "./auth.css";

export function Auth({
  busy,
  error,
  invited,
  submit,
}: {
  busy: boolean;
  error: string;
  invited: boolean;
  submit: (body: unknown, register: boolean) => void;
}) {
  const [register, setRegister] = useState(false);
  const [validationError, setValidationError] = useState("");
  const message = validationError || error;
  const changeMode = (value: boolean) => {
    setRegister(value);
    setValidationError("");
  };
  return (
    <main className="login-page">
      <section className="login-brand">
        <div className="login-hero" aria-hidden="true">
          <img
            src={publishedImageURL("/_common/login/hero.webp")}
            width="1373"
            height="1717"
            alt=""
            fetchPriority="high"
          />
        </div>
        <a className="login-logo" href="/" aria-label="围桌首页">
          <span className="login-logo-mark" aria-hidden="true">
            <span />
            <span />
            <span />
            <span />
          </span>
          <span>
            围桌<small>WIRE BOARD</small>
          </span>
        </a>
        <div className="login-copy">
          <p className="login-eyebrow">A Table For Every Game</p>
          <h1>
            一桌装下，
            <br />
            <span>所有好游戏</span>。
          </h1>
          <p className="login-description">
            卡牌策略、棋盘桌游、街机动作、休闲聚会，各类游戏点开就玩，随时随地开局。
          </p>
        </div>
        <div className="login-tags">
          {["卡牌策略", "棋盘桌游", "街机动作", "休闲聚会"].map((label) => (
            <span key={label}>{label}</span>
          ))}
        </div>
        <p className="login-brand-footer">
          Your Friends · Your Table · Your Next Move
        </p>
      </section>
      <section className="login-form">
        <div className="login-form-inner">
          <p className="login-form-eyebrow">Welcome To Wire Board</p>
          <h2>{register ? "创建新账号，开局吧。" : "欢迎回来，入座吧。"}</h2>
          <p className="login-subtitle">
            {register
              ? "使用朋友分享的邀请码，加入这个私人游戏大厅。"
              : invited
                ? "朋友在牌桌等你，登录后将自动进入邀请的房间。"
                : "桌布铺好了，就等你和朋友们。"}
          </p>
          <div className="login-tabs">
            <button
              type="button"
              aria-pressed={!register}
              className={!register ? "active" : ""}
              disabled={busy}
              onClick={() => changeMode(false)}
            >
              登录
            </button>
            <button
              type="button"
              aria-pressed={register}
              className={register ? "active" : ""}
              disabled={busy}
              onClick={() => changeMode(true)}
            >
              创建账号
            </button>
          </div>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (busy) return;
              const fields = new FormData(event.currentTarget);
              if (
                register &&
                fields.get("password") !== fields.get("passwordConfirm")
              ) {
                setValidationError("两次输入的密码不一致，请重新输入。");
                return;
              }
              setValidationError("");
              fields.delete("passwordConfirm");
              submit(Object.fromEntries(fields.entries()), register);
            }}
          >
            <label className="login-field">
              玩家昵称
              <span className="login-input">
                <UserRound size={18} aria-hidden="true" />
                <input
                  name="name"
                  placeholder="输入你的玩家昵称"
                  required
                  minLength={2}
                  maxLength={20}
                  autoComplete="username"
                />
              </span>
            </label>
            <label className="login-field">
              密码
              <span className="login-input">
                <Lock size={18} aria-hidden="true" />
                <input
                  name="password"
                  type="password"
                  placeholder="至少 8 位"
                  required
                  minLength={8}
                  maxLength={72}
                  autoComplete={register ? "new-password" : "current-password"}
                />
              </span>
            </label>
            {register && (
              <>
                <label className="login-field">
                  确认密码
                  <span className="login-input">
                    <Lock size={18} aria-hidden="true" />
                    <input
                      name="passwordConfirm"
                      type="password"
                      placeholder="再输入一次密码"
                      required
                      minLength={8}
                      maxLength={72}
                      autoComplete="new-password"
                    />
                  </span>
                </label>
                <label className="login-field">
                  邀请码
                  <span className="login-input">
                    <Ticket size={18} aria-hidden="true" />
                    <input
                      name="invite"
                      placeholder="向朋友获取邀请码"
                      required
                      autoComplete="off"
                    />
                  </span>
                </label>
              </>
            )}
            {message && (
              <p className="login-error" role="alert">
                {message}
              </p>
            )}
            <button type="submit" className="login-submit" disabled={busy}>
              {busy ? "请稍候…" : register ? "创建账号，入座" : "进入游戏大厅"}
              <ArrowRight size={18} aria-hidden="true" />
            </button>
          </form>
          <div className="login-footer">
            <a href="/releases" data-release-entry>
              更新日志
            </a>
            <span>
              <Lock size={14} aria-hidden="true" />
              只属于你和朋友的游戏空间
            </span>
          </div>
        </div>
      </section>
    </main>
  );
}
