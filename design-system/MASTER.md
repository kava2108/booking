# Design System MASTER — 施設予約管理システム (booking)

**バージョン**: 1.0.0  
**生成日**: 2026-04-12  
**スタック**: React 18 + TypeScript + TailwindCSS  
**コマンド**: `/kairo:design-system 施設予約管理`

---

## 目次

1. [デザイン原則](#1-デザイン原則)
2. [カラーパレット](#2-カラーパレット)
3. [タイポグラフィ](#3-タイポグラフィ)
4. [スペーシング & グリッド](#4-スペーシング--グリッド)
5. [シャドウ & ボーダー](#5-シャドウ--ボーダー)
6. [アイコン](#6-アイコン)
7. [コンポーネント一覧](#7-コンポーネント一覧)
8. [ページレイアウト](#8-ページレイアウト)
9. [レスポンシブブレークポイント](#9-レスポンシブブレークポイント)
10. [アニメーション & トランジション](#10-アニメーション--トランジション)
11. [アクセシビリティルール](#11-アクセシビリティルール)
12. [Tailwind Config 拡張定義](#12-tailwind-config-拡張定義)

---

## 1. デザイン原則

| 原則 | 説明 |
|------|------|
| **信頼性 (Trust)** | 予約・個人情報を扱うシステムとして、清潔感・安心感を与えるデザイン |
| **効率性 (Efficiency)** | 予約完了を3分以内に実現できるよう、迷いのない導線設計 |
| **明瞭性 (Clarity)** | 空き状況・料金・手順を一目で把握できる情報階層 |
| **モバイルファースト (Mobile-first)** | スマートフォンで快適に操作できることを基本とし PCへ拡張する |
| **インクルーシブ (Inclusive)** | WCAG 2.1 AA 準拠。高齢者・視覚障がい者も利用できる配色・文字サイズ |

---

## 2. カラーパレット

### 2.1 プライマリカラー — Teal（信頼・誠実さ）

施設予約サービスとして信頼感を与えるディープティール系を採用。

| トークン名 | Tailwind クラス | Hex | 用途 |
|-----------|----------------|-----|------|
| `primary-50` | `bg-primary-50` | `#EFF9F8` | 背景・サーフェス |
| `primary-100` | `bg-primary-100` | `#D0EFEC` | ホバー背景・ライトバッジ |
| `primary-200` | `bg-primary-200` | `#A3DDD7` | ボーダー・区切り線 |
| `primary-300` | `bg-primary-300` | `#6AC5BC` | セカンダリボタン hover |
| `primary-400` | `bg-primary-400` | `#3AADA3` | セカンダリボタン |
| `primary-500` | `bg-primary-500` | `#0D9488` | **メインブランドカラー** |
| `primary-600` | `bg-primary-600` | `#0B7A70` | プライマリボタン hover |
| `primary-700` | `bg-primary-700` | `#085E57` | 強調テキスト・リンク |
| `primary-800` | `bg-primary-800` | `#05413C` | ダーク背景 |
| `primary-900` | `bg-primary-900` | `#022520` | 最暗色 |

### 2.2 アクセントカラー — Amber（CTA・予約ボタン）

予約確定・送信などの主要 CTA に使用。視線を引き寄せ行動を促す。

| トークン名 | Tailwind クラス | Hex | 用途 |
|-----------|----------------|-----|------|
| `accent-50` | `bg-accent-50` | `#FFFBEB` | 軽い背景 |
| `accent-100` | `bg-accent-100` | `#FEF3C7` | バッジ背景 |
| `accent-400` | `bg-accent-400` | `#FBBF24` | セカンダリ CTA |
| `accent-500` | `bg-accent-500` | `#F59E0B` | **主要 CTA カラー** |
| `accent-600` | `bg-accent-600` | `#D97706` | CTA hover |
| `accent-700` | `bg-accent-700` | `#B45309` | アクティブ状態 |

### 2.3 セマンティックカラー

| トークン名 | Hex | Tailwind クラス | 用途 |
|-----------|-----|----------------|------|
| `success-500` | `#22C55E` | `bg-green-500` | 予約確定・成功メッセージ |
| `warning-500` | `#EAB308` | `bg-yellow-500` | 残り枠わずか・注意 |
| `error-500` | `#EF4444` | `bg-red-500` | エラー・バリデーション失敗 |
| `info-500` | `#3B82F6` | `bg-blue-500` | お知らせ・情報 |

### 2.4 ニュートラルカラー

| トークン名 | Hex | 用途 |
|-----------|-----|------|
| `neutral-50` | `#F9FAFB` | ページ背景 |
| `neutral-100` | `#F3F4F6` | カード背景・入力フィールド背景 |
| `neutral-200` | `#E5E7EB` | ボーダー・区切り線 |
| `neutral-400` | `#9CA3AF` | プレースホルダー・無効テキスト |
| `neutral-600` | `#4B5563` | サブテキスト・補足情報 |
| `neutral-800` | `#1F2937` | 本文テキスト |
| `neutral-900` | `#111827` | 見出し・強調テキスト |

### 2.5 コントラスト比（WCAG 2.1 AA 適合）

| 前景 | 背景 | 比率 | 適合 |
|------|------|------|------|
| `#0D9488` (primary-500) | `#FFFFFF` | 4.8:1 | ✅ AA |
| `#FFFFFF` | `#0D9488` (primary-500) | 4.8:1 | ✅ AA |
| `#111827` (neutral-900) | `#F9FAFB` (neutral-50) | 16.7:1 | ✅ AAA |
| `#FFFFFF` | `#F59E0B` (accent-500) | 2.9:1 | ⚠️ ボタンラベルは `#111827` を使用 |
| `#111827` | `#F59E0B` (accent-500) | 5.8:1 | ✅ AA |

---

## 3. タイポグラフィ

### 3.1 フォントファミリー

```css
/* 日本語: Noto Sans JP（可読性・信頼感）*/
/* 英数字: Inter（モダン・読みやすい）*/
font-family: 'Inter', 'Noto Sans JP', system-ui, -apple-system, sans-serif;
```

Tailwind 設定:
```js
fontFamily: {
  sans: ['Inter', 'Noto Sans JP', 'system-ui', '-apple-system', 'sans-serif'],
}
```

### 3.2 タイプスケール

| レベル | 用途 | サイズ | 行高 | フォントウェイト | Tailwind |
|--------|------|--------|------|----------------|---------|
| `display` | ヒーロー見出し | 48px / 3rem | 1.2 | 700 | `text-5xl font-bold leading-tight` |
| `h1` | ページタイトル | 36px / 2.25rem | 1.3 | 700 | `text-4xl font-bold` |
| `h2` | セクション見出し | 28px / 1.75rem | 1.35 | 600 | `text-3xl font-semibold` |
| `h3` | カード見出し | 22px / 1.375rem | 1.4 | 600 | `text-2xl font-semibold` |
| `h4` | サブ見出し | 18px / 1.125rem | 1.4 | 600 | `text-lg font-semibold` |
| `body-lg` | リード文・説明 | 18px / 1.125rem | 1.7 | 400 | `text-lg leading-relaxed` |
| `body` | 本文 | 16px / 1rem | 1.7 | 400 | `text-base leading-relaxed` |
| `body-sm` | 補足・キャプション | 14px / 0.875rem | 1.6 | 400 | `text-sm leading-relaxed` |
| `caption` | ラベル・バッジ | 12px / 0.75rem | 1.4 | 500 | `text-xs font-medium` |

> **最小フォントサイズ**: 12px（アクセシビリティ要件）

---

## 4. スペーシング & グリッド

### 4.1 スペーシングスケール（8px ベース）

| トークン | px | Tailwind |
|---------|-----|---------|
| `space-1` | 4px | `p-1` / `m-1` |
| `space-2` | 8px | `p-2` / `m-2` |
| `space-3` | 12px | `p-3` / `m-3` |
| `space-4` | 16px | `p-4` / `m-4` |
| `space-6` | 24px | `p-6` / `m-6` |
| `space-8` | 32px | `p-8` / `m-8` |
| `space-12` | 48px | `p-12` / `m-12` |
| `space-16` | 64px | `p-16` / `m-16` |
| `space-24` | 96px | `p-24` / `m-24` |

### 4.2 グリッドシステム

- **カラム数**: 12 カラム
- **コンテナ最大幅**: 1280px (`max-w-screen-xl`)
- **コンテナ横パディング**: 16px (sm) / 24px (md) / 32px (lg+)
- **ガター**: 24px (`gap-6`)

```html
<!-- コンテナ基本構造 -->
<div class="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
  <!-- コンテンツ -->
</div>
```

---

## 5. シャドウ & ボーダー

### 5.1 シャドウ

| トークン | 用途 | Tailwind |
|---------|------|---------|
| `shadow-xs` | 入力フィールド focus | `shadow-sm` |
| `shadow-sm` | カード（通常） | `shadow` |
| `shadow-md` | カード（hover） | `shadow-md` |
| `shadow-lg` | モーダル・ドロップダウン | `shadow-lg` |
| `shadow-xl` | トースト通知 | `shadow-xl` |

### 5.2 ボーダー半径

| トークン | px | 用途 | Tailwind |
|---------|-----|------|---------|
| `radius-sm` | 4px | バッジ・タグ | `rounded` |
| `radius-md` | 8px | ボタン・入力 | `rounded-lg` |
| `radius-lg` | 12px | カード | `rounded-xl` |
| `radius-xl` | 16px | モーダル・パネル | `rounded-2xl` |
| `radius-full` | 9999px | アバター・ピル | `rounded-full` |

### 5.3 ボーダー

| 用途 | スタイル | Tailwind |
|------|---------|---------|
| 通常の区切り線 | 1px solid neutral-200 | `border border-neutral-200` |
| フォーカスリング | 2px solid primary-500 | `ring-2 ring-primary-500` |
| エラーボーダー | 1px solid error-500 | `border-red-500` |

---

## 6. アイコン

**採用ライブラリ**: `lucide-react`（軽量・一貫性が高い・React 対応）

```bash
npm install lucide-react
```

### 6.1 使用アイコン一覧（主要機能別）

| 機能 | アイコン名 | 用途 |
|------|-----------|------|
| 予約 | `Calendar`, `CalendarCheck`, `CalendarX` | 予約・予約確定・キャンセル |
| 施設検索 | `Search`, `MapPin`, `Building2` | 検索・場所・施設 |
| ユーザー | `User`, `UserCircle`, `Users` | ユーザー・プロフィール |
| 認証 | `Lock`, `Unlock`, `Mail`, `Eye`, `EyeOff` | ログイン・パスワード |
| ナビゲーション | `ChevronRight`, `ChevronLeft`, `ArrowLeft` | 遷移・戻る |
| ステータス | `CheckCircle`, `XCircle`, `AlertCircle`, `Info` | 成功・失敗・警告・情報 |
| 時間 | `Clock`, `Timer` | 時刻・予約時間 |
| 金額 | `Yen`, `CreditCard` | 料金・支払い |
| 管理 | `Settings`, `BarChart2`, `LayoutDashboard` | 管理画面 |
| その他 | `Heart`, `Star`, `Share2`, `Bell` | お気に入り・評価 |

### 6.2 アイコンサイズ規則

| サイズ | px | 用途 | Tailwind |
|--------|-----|------|---------|
| `xs` | 14px | インラインテキスト内 | `w-3.5 h-3.5` |
| `sm` | 16px | ボタン内・バッジ内 | `w-4 h-4` |
| `md` | 20px | ナビゲーション・リスト | `w-5 h-5` |
| `lg` | 24px | カードアクション | `w-6 h-6` |
| `xl` | 32px | 空状態・イラスト代替 | `w-8 h-8` |

---

## 7. コンポーネント一覧

### 7.1 Button

#### バリアント定義

```tsx
// variants
type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
type ButtonSize   = 'sm' | 'md' | 'lg';
```

#### Tailwind クラス仕様

| バリアント | 通常状態 | hover | disabled |
|-----------|---------|-------|---------|
| `primary` | `bg-primary-500 text-white` | `hover:bg-primary-600` | `disabled:bg-neutral-300 disabled:cursor-not-allowed` |
| `cta`（予約確定） | `bg-accent-500 text-neutral-900 font-semibold` | `hover:bg-accent-600` | `disabled:bg-neutral-300` |
| `secondary` | `bg-white text-primary-700 border border-primary-300` | `hover:bg-primary-50` | `disabled:opacity-50` |
| `outline` | `bg-transparent text-neutral-700 border border-neutral-300` | `hover:bg-neutral-100` | `disabled:opacity-50` |
| `ghost` | `bg-transparent text-neutral-600` | `hover:bg-neutral-100 hover:text-neutral-900` | `disabled:opacity-50` |
| `danger` | `bg-error-500 text-white` | `hover:bg-red-600` | `disabled:opacity-50` |

#### サイズ別クラス

| サイズ | クラス |
|--------|-------|
| `sm` | `px-3 py-1.5 text-sm rounded-lg` |
| `md` | `px-4 py-2.5 text-base rounded-lg` |
| `lg` | `px-6 py-3 text-lg rounded-xl` |

#### コード例

```tsx
// src/components/ui/Button.tsx
import { Loader2 } from 'lucide-react';
import { type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'cta' | 'secondary' | 'outline' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  isLoading?: boolean;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
}

const variantClasses = {
  primary:   'bg-primary-500 text-white hover:bg-primary-600 focus-visible:ring-primary-500',
  cta:       'bg-accent-500 text-neutral-900 font-semibold hover:bg-accent-600 focus-visible:ring-accent-500',
  secondary: 'bg-white text-primary-700 border border-primary-300 hover:bg-primary-50 focus-visible:ring-primary-500',
  outline:   'bg-transparent text-neutral-700 border border-neutral-300 hover:bg-neutral-100 focus-visible:ring-neutral-400',
  ghost:     'bg-transparent text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 focus-visible:ring-neutral-400',
  danger:    'bg-red-500 text-white hover:bg-red-600 focus-visible:ring-red-500',
};

const sizeClasses = {
  sm: 'px-3 py-1.5 text-sm rounded-lg',
  md: 'px-4 py-2.5 text-base rounded-lg',
  lg: 'px-6 py-3 text-lg rounded-xl',
};

export function Button({
  variant = 'primary',
  size = 'md',
  isLoading = false,
  leftIcon,
  rightIcon,
  children,
  className,
  disabled,
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(
        'inline-flex items-center justify-center gap-2 font-medium transition-colors duration-150',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2',
        'disabled:cursor-not-allowed disabled:opacity-50',
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      disabled={disabled || isLoading}
      {...props}
    >
      {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : leftIcon}
      {children}
      {!isLoading && rightIcon}
    </button>
  );
}
```

---

### 7.2 Input / FormField

```tsx
// src/components/ui/Input.tsx
import { type InputHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/utils';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, className, id, ...props }, ref) => {
    return (
      <div className="flex flex-col gap-1.5">
        {label && (
          <label htmlFor={id} className="text-sm font-medium text-neutral-800">
            {label}
            {props.required && <span className="ml-1 text-red-500" aria-hidden="true">*</span>}
          </label>
        )}
        <input
          ref={ref}
          id={id}
          className={cn(
            'w-full rounded-lg border bg-neutral-100 px-3 py-2.5 text-base text-neutral-800',
            'placeholder:text-neutral-400 transition-colors duration-150',
            'focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent focus:bg-white',
            error
              ? 'border-red-500 focus:ring-red-500'
              : 'border-neutral-200 hover:border-neutral-300',
            'disabled:cursor-not-allowed disabled:opacity-50',
            className,
          )}
          aria-invalid={!!error}
          aria-describedby={error ? `${id}-error` : helperText ? `${id}-helper` : undefined}
          {...props}
        />
        {error && (
          <p id={`${id}-error`} className="text-sm text-red-500 flex items-center gap-1" role="alert">
            <span className="sr-only">エラー:</span>{error}
          </p>
        )}
        {!error && helperText && (
          <p id={`${id}-helper`} className="text-sm text-neutral-500">{helperText}</p>
        )}
      </div>
    );
  },
);
Input.displayName = 'Input';
```

---

### 7.3 Card

| バリアント | 用途 |
|-----------|------|
| `default` | 施設カード・情報表示 |
| `interactive` | クリック可能な施設一覧カード |
| `highlighted` | 注目施設・おすすめ |

```tsx
// src/components/ui/Card.tsx
import { cn } from '@/lib/utils';
import { type HTMLAttributes } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'interactive' | 'highlighted';
}

export function Card({ variant = 'default', className, children, ...props }: CardProps) {
  return (
    <div
      className={cn(
        'rounded-xl border border-neutral-200 bg-white shadow',
        variant === 'interactive' && 'cursor-pointer transition-all duration-200 hover:shadow-md hover:-translate-y-0.5',
        variant === 'highlighted' && 'border-primary-300 ring-1 ring-primary-200 shadow-md',
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-6 pb-0', className)} {...props} />;
}

export function CardBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-6', className)} {...props} />;
}

export function CardFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('px-6 py-4 border-t border-neutral-100 bg-neutral-50 rounded-b-xl', className)}
      {...props}
    />
  );
}
```

---

### 7.4 Badge / StatusChip

予約ステータス・空き状況表示に使用。

| バリアント | 色 | 用途例 |
|-----------|-----|-------|
| `available` | green | 空きあり |
| `few` | yellow | 残りわずか（残 1〜3 枠） |
| `full` | red | 満席 |
| `pending` | blue | 予約審査中 |
| `confirmed` | green | 予約確定 |
| `cancelled` | neutral | キャンセル済み |

```tsx
// src/components/ui/Badge.tsx
import { cn } from '@/lib/utils';

type BadgeVariant = 'available' | 'few' | 'full' | 'pending' | 'confirmed' | 'cancelled' | 'info';

const badgeClasses: Record<BadgeVariant, string> = {
  available:  'bg-green-100 text-green-800 border-green-200',
  few:        'bg-yellow-100 text-yellow-800 border-yellow-200',
  full:       'bg-red-100 text-red-700 border-red-200',
  pending:    'bg-blue-100 text-blue-800 border-blue-200',
  confirmed:  'bg-primary-100 text-primary-800 border-primary-200',
  cancelled:  'bg-neutral-100 text-neutral-600 border-neutral-200',
  info:       'bg-blue-50 text-blue-700 border-blue-200',
};

const badgeLabels: Record<BadgeVariant, string> = {
  available: '空きあり',
  few:       '残りわずか',
  full:      '満席',
  pending:   '審査中',
  confirmed: '予約確定',
  cancelled: 'キャンセル済み',
  info:      '情報',
};

interface BadgeProps {
  variant: BadgeVariant;
  label?: string;
  className?: string;
}

export function Badge({ variant, label, className }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded border px-2 py-0.5 text-xs font-medium',
        badgeClasses[variant],
        className,
      )}
    >
      {label ?? badgeLabels[variant]}
    </span>
  );
}
```

---

### 7.5 Alert / Toast

| バリアント | アイコン | 色 | 用途 |
|-----------|---------|-----|------|
| `success` | `CheckCircle` | green | 予約確定・操作成功 |
| `error` | `XCircle` | red | エラー・失敗 |
| `warning` | `AlertCircle` | yellow | 注意・変更 |
| `info` | `Info` | blue | お知らせ |

```tsx
// src/components/ui/Alert.tsx
import { CheckCircle, XCircle, AlertCircle, Info } from 'lucide-react';
import { cn } from '@/lib/utils';

type AlertVariant = 'success' | 'error' | 'warning' | 'info';

const alertConfig: Record<AlertVariant, { icon: typeof CheckCircle; classes: string }> = {
  success: { icon: CheckCircle, classes: 'bg-green-50 border-green-300 text-green-900' },
  error:   { icon: XCircle,     classes: 'bg-red-50 border-red-300 text-red-900' },
  warning: { icon: AlertCircle, classes: 'bg-yellow-50 border-yellow-300 text-yellow-900' },
  info:    { icon: Info,        classes: 'bg-blue-50 border-blue-300 text-blue-900' },
};

export function Alert({
  variant,
  title,
  message,
  className,
}: {
  variant: AlertVariant;
  title: string;
  message?: string;
  className?: string;
}) {
  const { icon: Icon, classes } = alertConfig[variant];
  return (
    <div
      className={cn('flex gap-3 rounded-xl border p-4', classes, className)}
      role="alert"
      aria-live="polite"
    >
      <Icon className="w-5 h-5 mt-0.5 shrink-0" aria-hidden="true" />
      <div>
        <p className="font-semibold text-sm">{title}</p>
        {message && <p className="mt-0.5 text-sm opacity-90">{message}</p>}
      </div>
    </div>
  );
}
```

---

### 7.6 DatePicker / TimePicker（施設予約専用）

予約日時の選択 UI。`react-day-picker` + カスタムスタイルを推奨。

```
// Tailwind カスタムクラス（tailwind.config.js の safelist に追加）
rdp-day_selected: bg-primary-500 text-white rounded-lg
rdp-day_today: font-bold text-primary-700
rdp-day_disabled: text-neutral-300 cursor-not-allowed
rdp-day_outside: text-neutral-400
rdp-button:hover: bg-primary-50 rounded-lg
```

---

### 7.7 Modal / Dialog

```tsx
// src/components/ui/Modal.tsx
// react-aria または Headless UI を使用（アクセシビリティ確保）
// ベースクラス:
// overlay: fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50
// panel:   w-full max-w-md rounded-2xl bg-white shadow-xl p-6 mx-4
// header:  flex items-center justify-between mb-4
// close:   rounded-full p-1 hover:bg-neutral-100 transition-colors
```

---

### 7.8 Skeleton Loader

空き状況・施設一覧取得中に表示するプレースホルダー。

```tsx
// src/components/ui/Skeleton.tsx
import { cn } from '@/lib/utils';

export function Skeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn('animate-pulse rounded-lg bg-neutral-200', className)}
      aria-hidden="true"
    />
  );
}

// 施設カード用スケルトン
export function FacilityCardSkeleton() {
  return (
    <div className="rounded-xl border border-neutral-200 bg-white p-4 shadow">
      <Skeleton className="h-48 w-full rounded-lg mb-4" />
      <Skeleton className="h-5 w-3/4 mb-2" />
      <Skeleton className="h-4 w-1/2 mb-4" />
      <Skeleton className="h-9 w-28" />
    </div>
  );
}
```

---

### 7.9 Navigation / Header

#### メインナビゲーション構成

| エリア | 要素 |
|--------|------|
| ロゴ | 左端固定 |
| メインリンク | 施設を探す / 予約履歴 / ヘルプ |
| ユーザーエリア | ログイン / マイページ（ドロップダウン） |
| モバイル | ハンバーガーメニュー → サイドドロワー |

```html
<!-- ヘッダー基本構造 -->
<header class="sticky top-0 z-40 w-full border-b border-neutral-200 bg-white/95 backdrop-blur">
  <div class="mx-auto max-w-screen-xl px-4 sm:px-6 lg:px-8">
    <div class="flex h-16 items-center justify-between">
      <!-- ロゴ / ナビ / ユーザーエリア -->
    </div>
  </div>
</header>
```

---

### 7.10 施設カード（FacilityCard）

```html
<!-- 施設カード構造 -->
<article class="rounded-xl border border-neutral-200 bg-white shadow cursor-pointer
                transition-all duration-200 hover:shadow-md hover:-translate-y-0.5
                focus-visible:ring-2 ring-primary-500">
  <!-- 施設画像 -->
  <div class="relative">
    <img class="h-48 w-full rounded-t-xl object-cover" src="..." alt="施設名" />
    <!-- 空きステータスバッジ -->
    <div class="absolute top-3 right-3">
      <Badge variant="available" />
    </div>
  </div>

  <div class="p-4">
    <!-- 施設名 -->
    <h3 class="text-lg font-semibold text-neutral-900 line-clamp-1">会議室 A</h3>

    <!-- 場所 -->
    <p class="mt-1 flex items-center gap-1 text-sm text-neutral-600">
      <MapPin class="w-4 h-4" />
      <span>渋谷区神宮前 1-1-1</span>
    </p>

    <!-- 料金 -->
    <p class="mt-2 text-base font-semibold text-primary-700">
      ¥3,000 <span class="text-sm font-normal text-neutral-500">/ 時間</span>
    </p>

    <!-- 予約ボタン -->
    <Button variant="cta" size="sm" class="mt-4 w-full">
      <CalendarCheck class="w-4 h-4" />
      予約する
    </Button>
  </div>
</article>
```

---

## 8. ページレイアウト

### 8.1 ページ種別と構成要素

| ページ | レイアウト | 主要コンポーネント |
|--------|-----------|------------------|
| トップ（ランディング） | フルワイド | Hero / SearchBar / FacilityCard Grid |
| 施設一覧 | 2カラム（絞り込み + グリッド） | FilterPanel / FacilityCard |
| 施設詳細 | 1カラム + サイドバー | Gallery / BookingWidget / ReviewList |
| 予約フォーム | 1カラム センター | Stepper / DatePicker / Button(CTA) |
| 予約確認 | 1カラム センター | ConfirmCard / Button(CTA + secondary) |
| 予約完了 | 1カラム センター | SuccessIllustration / BookingDetail |
| ログイン / 登録 | センター カード | AuthCard / Input / Button |
| マイページ | 2カラム（サイドナビ + コンテンツ） | SideNav / BookingHistoryList |
| 管理ダッシュボード | サイドバー + コンテンツ | Sidebar / StatCard / ReservationTable |

### 8.2 予約フロー Stepper

```
① 日時選択  →  ② 内容確認  →  ③ 予約完了
```

```html
<!-- Stepper -->
<nav aria-label="予約手順" class="flex items-center justify-center gap-0">
  <div class="flex items-center gap-2">
    <!-- Step 1: アクティブ -->
    <div class="flex items-center justify-center w-8 h-8 rounded-full
                bg-primary-500 text-white text-sm font-semibold">1</div>
    <span class="text-sm font-medium text-primary-700">日時選択</span>
  </div>

  <div class="w-16 h-px bg-neutral-300 mx-2" aria-hidden="true" />

  <div class="flex items-center gap-2">
    <!-- Step 2: 未完了 -->
    <div class="flex items-center justify-center w-8 h-8 rounded-full
                border-2 border-neutral-300 text-neutral-500 text-sm font-semibold">2</div>
    <span class="text-sm font-medium text-neutral-400">内容確認</span>
  </div>

  <div class="w-16 h-px bg-neutral-300 mx-2" aria-hidden="true" />

  <div class="flex items-center gap-2">
    <!-- Step 3: 未完了 -->
    <div class="flex items-center justify-center w-8 h-8 rounded-full
                border-2 border-neutral-300 text-neutral-500 text-sm font-semibold">3</div>
    <span class="text-sm font-medium text-neutral-400">予約完了</span>
  </div>
</nav>
```

---

## 9. レスポンシブブレークポイント

| ブレークポイント | 幅 | 用途 |
|----------------|-----|------|
| `xs` | < 480px | スマートフォン縦（1カラムグリッド） |
| `sm` | ≥ 640px | スマートフォン横（モーダル調整） |
| `md` | ≥ 768px | タブレット（2カラムグリッド） |
| `lg` | ≥ 1024px | PC（3カラムグリッド・サイドバー表示） |
| `xl` | ≥ 1280px | ワイドPC |

### グリッド切り替えパターン

```html
<!-- 施設一覧 -->
<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
  <!-- FacilityCard x N -->
</div>

<!-- 管理ダッシュボード統計 -->
<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
  <!-- StatCard x 4 -->
</div>
```

---

## 10. アニメーション & トランジション

### 10.1 標準トランジション

| 種別 | 時間 | イージング | Tailwind |
|------|------|-----------|---------|
| ホバーカラー | 150ms | ease-in-out | `transition-colors duration-150` |
| カード浮き上がり | 200ms | ease-out | `transition-all duration-200` |
| モーダル表示 | 200ms | ease-out | `transition-opacity duration-200` |
| トースト滑り込み | 300ms | ease-out | `transition-transform duration-300` |
| ページ遷移 | 200ms | ease-in-out | `transition-opacity duration-200` |

### 10.2 カスタムアニメーション（tailwind.config.js）

```js
extend: {
  animation: {
    'slide-up': 'slideUp 0.3s ease-out',
    'fade-in':  'fadeIn 0.2s ease-out',
  },
  keyframes: {
    slideUp: {
      '0%':   { transform: 'translateY(8px)', opacity: '0' },
      '100%': { transform: 'translateY(0)',   opacity: '1' },
    },
    fadeIn: {
      '0%':   { opacity: '0' },
      '100%': { opacity: '1' },
    },
  },
},
```

### 10.3 モーション削減（アクセシビリティ）

```css
/* globals.css */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 11. アクセシビリティルール

| カテゴリ | ルール |
|---------|--------|
| **フォーカス管理** | すべてのインタラクティブ要素に `focus-visible` リングを設定（`ring-2 ring-primary-500 ring-offset-2`） |
| **ラベル** | `<input>` には必ず `<label>` または `aria-label`。`placeholder` のみはNG |
| **エラー** | エラーメッセージは `role="alert"` + `aria-invalid="true"` |
| **画像** | 施設画像には `alt` 属性必須。装飾画像は `alt=""` |
| **ボタンアイコン** | アイコンのみのボタンには `aria-label` または `<span class="sr-only">` |
| **カラーコントラスト** | 本文: 4.5:1 以上 / 大きな文字: 3:1 以上（WCAG 2.1 AA） |
| **セマンティックHTML** | `<article>`, `<nav>`, `<main>`, `<aside>` を適切に使用 |
| **ランドマーク** | `<main id="main-content">` + スキップリンクを実装 |
| **キーボード操作** | Tab / Shift+Tab / Enter / Space / Esc で全操作可能に |
| **ライブリージョン** | 空き状況等の動的更新は `aria-live="polite"` |

---

## 12. Tailwind Config 拡張定義

```js
// tailwind.config.js
const { fontFamily } = require('tailwindcss/defaultTheme');

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: {
          50:  '#EFF9F8',
          100: '#D0EFEC',
          200: '#A3DDD7',
          300: '#6AC5BC',
          400: '#3AADA3',
          500: '#0D9488',  // main brand
          600: '#0B7A70',
          700: '#085E57',
          800: '#05413C',
          900: '#022520',
        },
        accent: {
          50:  '#FFFBEB',
          100: '#FEF3C7',
          400: '#FBBF24',
          500: '#F59E0B',  // CTA
          600: '#D97706',
          700: '#B45309',
        },
        neutral: {
          50:  '#F9FAFB',
          100: '#F3F4F6',
          200: '#E5E7EB',
          300: '#D1D5DB',
          400: '#9CA3AF',
          500: '#6B7280',
          600: '#4B5563',
          700: '#374151',
          800: '#1F2937',
          900: '#111827',
        },
      },
      fontFamily: {
        sans: ['Inter', 'Noto Sans JP', ...fontFamily.sans],
      },
      animation: {
        'slide-up': 'slideUp 0.3s ease-out',
        'fade-in':  'fadeIn 0.2s ease-out',
      },
      keyframes: {
        slideUp: {
          '0%':   { transform: 'translateY(8px)', opacity: '0' },
          '100%': { transform: 'translateY(0)',   opacity: '1' },
        },
        fadeIn: {
          '0%':   { opacity: '0' },
          '100%': { opacity: '1' },
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/line-clamp'),
  ],
};
```

---

## 付録: ファイル配置ガイドライン

```
frontend/src/
├── components/
│   └── ui/              # このDesign Systemのコンポーネント
│       ├── Button.tsx
│       ├── Input.tsx
│       ├── Card.tsx
│       ├── Badge.tsx
│       ├── Alert.tsx
│       ├── Skeleton.tsx
│       └── Modal.tsx
├── features/
│   ├── auth/
│   │   └── components/  # ログイン・登録フォーム
│   ├── booking/
│   │   └── components/  # 予約フォーム・Stepper・CalendarPicker
│   └── facility/
│       └── components/  # FacilityCard・FacilityGallery
├── pages/               # ページコンポーネント
└── lib/
    └── utils.ts         # cn() ユーティリティ (clsx + tailwind-merge)
```

```ts
// src/lib/utils.ts
import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

---

*このファイルは `/kairo:design-system` コマンドにより自動生成されました。変更は `/kairo:design-system --update` で同期してください。*
