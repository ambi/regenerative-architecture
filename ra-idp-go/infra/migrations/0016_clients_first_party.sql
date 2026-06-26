-- ADR-061: IdP 自身が所有する信頼済みクライアント (管理コンソール /
-- アカウントポータル) を表すフラグ。resource owner が IdP 利用者自身であるため、
-- authorization_code フローで consent 画面をスキップする。既存クライアントは
-- サードパーティ扱い (FALSE) を既定とする。

ALTER TABLE clients
    ADD COLUMN IF NOT EXISTS first_party BOOLEAN NOT NULL DEFAULT FALSE;
