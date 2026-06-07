/**
 * 翻訳カタログ。ja を canonical schema として、en は同じ shape で書く。
 *
 * 直接ネストでアクセスする (`m.login.title`) ことで:
 *   - キーミスを TS の型でブロックできる
 *   - i18n ランタイム (react-i18next 等) を入れずに ~0 byte の追加で済む
 *   - 文言追加は型を見ながら両ロケールを揃えるので drift しにくい
 *
 * Phase 5 (テナント) で文言の上書きを許す場合は、サーバが meta タグで
 * 個別キーの上書きを送り、本ファイルとマージする slot を入れる。
 */

const ja = {
  appName: 'RA IdP',
  brand: {
    secure: 'TLS で保護',
    pending: '入力待ち',
    error: 'エラー',
  },
  layout: {
    skipToContent: 'メインコンテンツへ移動',
    footerLeft: 'RA IdP / regenerative-architecture',
    footerRight: 'OpenID Connect · OAuth 2.0',
  },
  login: {
    title: 'サインイン',
    description: 'アカウントの情報を入力してください',
    username: 'ユーザー名',
    password: 'パスワード',
    submit: 'サインイン',
    submitting: 'サインイン中…',
    errorTitle: 'サインインできません',
    errorBody: 'ユーザー名またはパスワードが正しくありません。',
    invalidRequestTitle: '認可リクエストが見つかりません',
    invalidRequestBody:
      '認可フローのコンテキストが見つからないか、有効期限が切れています。アプリケーションから OAuth フローを再開してください。',
    networkError: '通信エラーが発生しました。時間をおいて再試行してください。',
    footer: '不正なログイン試行は',
    footerCode: 'audit log',
    footerTail: 'に記録されます',
  },
  consent: {
    title: 'アクセス許可',
    descriptionPrefix: '',
    descriptionSuffix: ' があなたのアカウント情報にアクセスしようとしています',
    requestedHeading: '要求されている権限',
    deviceRequesting: '物理デバイスから要求されています',
    physicalHint: 'テレビ・CLI ツール・スマート家電などのブラウザを持たないデバイスです',
    allow: '許可する',
    deny: '拒否する',
    errorTitle: 'エラー',
    errorBody: 'リクエストの処理に失敗しました。',
    networkError: '通信エラーが発生しました。時間をおいて再試行してください。',
    footer: '許可した内容は',
    footerCode: 'consent ledger',
    footerTail: 'に記録され、いつでも取り消せます',
    scopes: {
      openid: { title: '識別子', description: 'あなたの一意なユーザー ID' },
      profile: { title: 'プロフィール', description: '氏名・ユーザー名などの基本プロフィール情報' },
      email: { title: 'メールアドレス', description: 'メールアドレスと検証ステータス' },
      address: { title: '住所', description: '登録された住所' },
      phone: { title: '電話番号', description: '電話番号と検証ステータス' },
      offline_access: {
        title: 'オフラインアクセス',
        description: 'ログアウト後もアプリがバックグラウンドで動作できるようにする',
      },
      unknown: { title: '', description: 'アプリ固有のアクセス権' },
    },
  },
  device: {
    title: 'デバイスを認可',
    description: 'デバイスに表示されたコードを入力してください',
    userCodeLabel: 'user code',
    placeholder: 'XXXX-XXXX',
    allow: '認可する',
    deny: '拒否する',
    deviceRequesting: '物理デバイスから要求されています',
    physicalHint: 'テレビ・CLI ツール・スマート家電などのブラウザを持たないデバイスです',
    errorEmptyCode: 'user_code を入力してください。',
    errorTitle: '確認できませんでした',
    errorBody: 'リクエストの処理に失敗しました。コードを再確認してください。',
    networkError: '通信エラーが発生しました。',
    footer: 'デバイスの確認は ',
    footerCode: 'RFC 8628',
    footerTail: ' Device Authorization Grant に従います',
  },
  totp: {
    title: '第二要素の確認',
    description: 'Authenticator アプリに表示された 6 桁のコードを入力してください',
    codeLabel: '確認コード',
    placeholder: '000000',
    submit: '確認',
    submitting: '確認中…',
    errorTitle: 'コードを確認できませんでした',
    errorBody: 'コードが正しくないか、有効期限が切れています。',
    errorEmptyCode: 'コードを入力してください。',
    networkError: '通信エラーが発生しました。時間をおいて再試行してください。',
    footer: 'Authenticator アプリは ',
    footerCode: 'RFC 6238',
    footerTail: ' に従い 30 秒ごとに更新されます',
  },
  error: {
    detailFallback: '問題が解消しない場合は',
    detailFallbackCode: 'audit log',
    detailFallbackTail: 'を管理者に共有してください',
    variants: {
      logged_out: {
        title: 'ログアウトしました',
        description: 'セッションを終了しました。安全のためブラウザを閉じてください。',
      },
      access_denied: {
        title: 'アクセスが拒否されました',
        description: 'リクエストは管理者またはあなた自身によって拒否されました。',
      },
      device_approved: {
        title: 'デバイスを認可しました',
        description: 'デバイス側で続行できます。このタブは閉じてかまいません。',
      },
      device_denied: {
        title: 'デバイス認可を拒否しました',
        description: 'デバイスからのアクセス要求を拒否しました。',
      },
      invalid_request: {
        title: 'リクエストが無効です',
        description:
          'パラメータの形式または値が不正です。アプリケーションの管理者に連絡してください。',
      },
      login_required: {
        title: 'ログインが必要です',
        description: 'デモ環境では X-User-Sub ヘッダでユーザー識別を行います。',
      },
      default: {
        title: 'エラーが発生しました',
        description: '時間をおいて再試行してください。',
      },
    },
  },
} satisfies Messages

interface ScopeDescriptor {
  title: string
  description: string
}

interface ErrorVariantText {
  title: string
  description: string
}

interface Messages {
  appName: string
  brand: { secure: string; pending: string; error: string }
  layout: { skipToContent: string; footerLeft: string; footerRight: string }
  login: {
    title: string
    description: string
    username: string
    password: string
    submit: string
    submitting: string
    errorTitle: string
    errorBody: string
    invalidRequestTitle: string
    invalidRequestBody: string
    networkError: string
    footer: string
    footerCode: string
    footerTail: string
  }
  consent: {
    title: string
    descriptionPrefix: string
    descriptionSuffix: string
    requestedHeading: string
    deviceRequesting: string
    physicalHint: string
    allow: string
    deny: string
    errorTitle: string
    errorBody: string
    networkError: string
    footer: string
    footerCode: string
    footerTail: string
    scopes: {
      openid: ScopeDescriptor
      profile: ScopeDescriptor
      email: ScopeDescriptor
      address: ScopeDescriptor
      phone: ScopeDescriptor
      offline_access: ScopeDescriptor
      unknown: ScopeDescriptor
    }
  }
  device: {
    title: string
    description: string
    userCodeLabel: string
    placeholder: string
    allow: string
    deny: string
    deviceRequesting: string
    physicalHint: string
    errorEmptyCode: string
    errorTitle: string
    errorBody: string
    networkError: string
    footer: string
    footerCode: string
    footerTail: string
  }
  totp: {
    title: string
    description: string
    codeLabel: string
    placeholder: string
    submit: string
    submitting: string
    errorTitle: string
    errorBody: string
    errorEmptyCode: string
    networkError: string
    footer: string
    footerCode: string
    footerTail: string
  }
  error: {
    detailFallback: string
    detailFallbackCode: string
    detailFallbackTail: string
    variants: {
      logged_out: ErrorVariantText
      access_denied: ErrorVariantText
      device_approved: ErrorVariantText
      device_denied: ErrorVariantText
      invalid_request: ErrorVariantText
      login_required: ErrorVariantText
      default: ErrorVariantText
    }
  }
}

const en: Messages = {
  appName: 'RA IdP',
  brand: {
    secure: 'TLS secured',
    pending: 'awaiting input',
    error: 'error',
  },
  layout: {
    skipToContent: 'Skip to main content',
    footerLeft: 'RA IdP / regenerative-architecture',
    footerRight: 'OpenID Connect · OAuth 2.0',
  },
  login: {
    title: 'Sign in',
    description: 'Enter your account details to continue',
    username: 'Username',
    password: 'Password',
    submit: 'Sign in',
    submitting: 'Signing in…',
    errorTitle: 'Sign in failed',
    errorBody: 'Username or password is incorrect.',
    invalidRequestTitle: 'No authorization request',
    invalidRequestBody:
      'No active authorization context was found, or it has expired. Restart the OAuth flow from your application.',
    networkError: 'A network error occurred. Please try again shortly.',
    footer: 'Unsuccessful attempts are recorded to the ',
    footerCode: 'audit log',
    footerTail: '',
  },
  consent: {
    title: 'Authorize access',
    descriptionPrefix: '',
    descriptionSuffix: ' is requesting access to your account information',
    requestedHeading: 'Requested permissions',
    deviceRequesting: 'Request from a physical device',
    physicalHint: 'TVs, CLI tools, and smart appliances without a browser',
    allow: 'Allow',
    deny: 'Deny',
    errorTitle: 'Error',
    errorBody: 'Could not complete the request.',
    networkError: 'A network error occurred. Please try again shortly.',
    footer: 'Granted access is recorded to the ',
    footerCode: 'consent ledger',
    footerTail: ' and can be revoked at any time',
    scopes: {
      openid: { title: 'Identifier', description: 'Your unique user identifier' },
      profile: { title: 'Profile', description: 'Name, username, and basic profile information' },
      email: { title: 'Email', description: 'Email address and verification status' },
      address: { title: 'Address', description: 'Registered postal address' },
      phone: { title: 'Phone', description: 'Phone number and verification status' },
      offline_access: {
        title: 'Offline access',
        description: 'Allow the app to operate in the background after sign-out',
      },
      unknown: { title: '', description: 'Application-specific permission' },
    },
  },
  device: {
    title: 'Authorize device',
    description: 'Enter the code shown on your device',
    userCodeLabel: 'user code',
    placeholder: 'XXXX-XXXX',
    allow: 'Authorize',
    deny: 'Deny',
    deviceRequesting: 'Request from a physical device',
    physicalHint: 'TVs, CLI tools, and smart appliances without a browser',
    errorEmptyCode: 'Please enter the user_code.',
    errorTitle: 'Could not verify',
    errorBody: 'Could not complete the request. Please check the code.',
    networkError: 'A network error occurred.',
    footer: 'Device verification follows ',
    footerCode: 'RFC 8628',
    footerTail: ' Device Authorization Grant',
  },
  totp: {
    title: 'Verify second factor',
    description: 'Enter the 6-digit code shown in your authenticator app',
    codeLabel: 'Verification code',
    placeholder: '000000',
    submit: 'Verify',
    submitting: 'Verifying…',
    errorTitle: 'Could not verify code',
    errorBody: 'The code is incorrect or has expired.',
    errorEmptyCode: 'Please enter the code.',
    networkError: 'A network error occurred. Please try again shortly.',
    footer: 'Authenticator apps follow ',
    footerCode: 'RFC 6238',
    footerTail: ' and refresh every 30 seconds',
  },
  error: {
    detailFallback: 'If the problem persists, share the ',
    detailFallbackCode: 'audit log',
    detailFallbackTail: ' with an administrator',
    variants: {
      logged_out: {
        title: 'You have signed out',
        description: 'Your session has ended. Close this browser for safety.',
      },
      access_denied: {
        title: 'Access denied',
        description: 'The request was denied by you or an administrator.',
      },
      device_approved: {
        title: 'Device authorized',
        description: 'Continue from the device. You may close this tab.',
      },
      device_denied: {
        title: 'Device denied',
        description: 'The device authorization request was denied.',
      },
      invalid_request: {
        title: 'Invalid request',
        description: 'A parameter is malformed. Please contact the application owner.',
      },
      login_required: {
        title: 'Sign in required',
        description:
          'This demo identifies users via the X-User-Sub header. See the README for details.',
      },
      default: {
        title: 'An error occurred',
        description: 'Please try again shortly.',
      },
    },
  },
}

export const CATALOGS = { ja, en } as const
export type Locale = keyof typeof CATALOGS
export type { Messages }

export function pickCatalog(locale: string | null | undefined): Messages {
  return locale === 'en' ? en : ja
}
