# ADR-037: ユースケース層はドメインの仕事を担うときだけ通す

## ステータス

採用。

## コンテキスト

`internal/adapters/http/` の HTTP ハンドラーは、書き込み系
(Create / Update / Delete) では `internal/<bounded-context>/usecases` の
関数を経由する一方、読み取り系 (List / Get) では多くが
`d.ClientRepo.FindAll` / `d.UserRepo.FindBySub` のように Repository ポートを
直接呼んでいる。

「Clean Architecture / DDD 的にすべての HTTP ハンドラーがユースケース層を
経由するべきではないか」という問いは繰り返し出る。一方で、現在のクエリ系
ハンドラーをユースケース化すると本体が `return repo.X(ctx, ...)` の一行
ラッパーになり、層が一つ増える以上の効果が無い。

判断を都度 reviewer の主観に委ねると以下のドリフトが起きる。

- 一貫性のないラッパー (片方の List だけユースケース化、もう片方は直呼び)。
- 「念のため」のセレモニー層追加によって、関数定義・テスト・モックが
  実体無く倍増する。
- 逆に、本来ドメイン由来の絞り込みや認可をハンドラー側に書き散らすことで、
  use case 層が空洞化し責務境界が失われる。

決定の本質は「層の名前」ではなく「ハンドラーに置きたくない仕事があるか」
であり、これを ADR として固定する。

## 決定

1. **書き込み系 (state-changing) は原則ユースケース経由**。HTTP ハンドラーは
   入力デコード・presentational な認可 (`requireAdmin` 等)・出力 DTO 変換に
   留め、ドメイン不変条件・event 発行・複数 aggregate オーケストレーション・
   トランザクション境界はユースケース関数が担う。

2. **読み取り系 (query) は Repository 直接呼びを許容する**。クエリ用
   ユースケースを噛ませても本体が単一 Repository 呼び出しの薄いラッパー
   にしかならないなら、ユースケースは設けない。

3. **ユースケース化の判定軸**。次のいずれかを担うときに限り、当該クエリ
   経路にもユースケースを設ける。

   - ドメイン不変条件・状態遷移判定
   - ドメインイベント / 監査イベントの発行
   - 複数 Repository・複数 aggregate をまたぐオーケストレーション
   - トランザクション境界の調整
   - ドメイン由来の認可 (presentation 層の `requireAdmin` を超え、tenant・
     role・データ可視性などドメイン文脈に依存する判定)
   - 同じ問い合わせを HTTP・CLI・バッチなど複数の入口から再利用する必要

4. **ドリフト兆候が出たら昇格する**。直呼びだったクエリ経路に次が現れたら
   ユースケースに引き上げる。

   - 認可で「自分のテナントの分のみ」「stale を除外」等のドメイン文脈
     依存の絞り込みが追加される。
   - 複数 Repository を join する射影が必要になる。
   - 同一クエリを別の入口 (CLI・スケジューラ・別ハンドラー) が再利用する。
   - 読み取りに監査ログを残す要件が来る。

   予防的にユースケース層を厚くはしない (出てから引き上げる)。

5. **コマンド側でユースケースを省略しない**。書き込みが「現状は repo を一回
   呼ぶだけ」に見える場合でも、ドメインイベントの発行・時刻の引き渡し・
   actor の明示などで将来ユースケースが必要になる確率が高いため、最初から
   ユースケースを通す。

6. **境界の表現**。ハンドラーが呼べる依存は `Deps` 構造体に集約されており、
   そこに Repository ポートとユースケース関数を共存させること自体は許容
   する (Clean Architecture の「ハンドラーは内側のリングだけを参照」とは
   矛盾しない。Repository ポートはアダプタではなく内側で定義される
   インターフェースである)。

## 影響

- 既存の `handleListAdminClients` / `handleGetAdminClient` /
  `handleListAdminUsers` / `handleGetAdminUser` 等の直 Repository 呼び出しは
  本 ADR により正規化される (リファクタ不要)。
- 書き込み系で本 ADR の判定軸を満たさないままユースケースを跨いでいる
  経路があれば、新規 ADR 無しで簡素化してよい。逆に、クエリ経路で判定軸を
  満たすものが見つかれば本 ADR を根拠にユースケース化する。
- 新規ハンドラーを足す PR の review では「クエリだからユースケース不要」
  「コマンドだからユースケース経由」を出発点とし、判定軸の項目を満たすか
  どうかで例外判断する。
- CLAUDE.md には記載しない。ADR が原則の置き場所であり、CLAUDE.md は
  原則の運用方針 (commit hygiene 等) に留める。

## 却下した代替案

- **すべての HTTP ハンドラーをユースケース経由にする**。Clean Architecture の
  読み筋としてはあり得るが、本リポジトリでは薄いラッパーを大量に生む。
  CQRS 文献 (Vernon, Young) でもクエリ側はドメインモデルを経由する義務が
  ないとされる。形式の動物園を増やさないという方針 (Regenerative
  Architecture) とも整合しない。
- **すべての HTTP ハンドラーから Repository を直接呼ぶ**。書き込みで
  必要となるドメインイベント発行・トランザクション境界・複数 aggregate
  オーケストレーションをハンドラーに散らすことになり、ドメイン層が空洞化
  する。

## 関連

- ADR-016 (Persistence adapter selection): Repository ポートの所在。
- ADR-018 (Audit vs application log separation): event 発行はユースケース
  からとする原則。
- ADR-031 (Admin user API と RBAC): `requireAdmin` という presentation 層
  の認可境界。
- ADR-034 (Tenant-scoped persistence): tenant 文脈は Repository 引数で
  渡る。
