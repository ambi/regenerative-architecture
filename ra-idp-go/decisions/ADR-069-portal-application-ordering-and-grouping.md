# ADR-069: 利用者ポータルのアプリ並び替えと分類の所有

## ステータス
採用。`spec/contexts/application.yaml` と `internal/application/` に反映。
本 ADR は所有関係のみを定め、wire 挙動の規範は SCL が所有する。
並び替え (slice 1) と分類 (slice 2) を同じ所有方針で実装した。

## コンテキスト
wi-69 / ADR-064 で利用者ポータルに割当済みアプリのタイル一覧が入った。
一覧は ApplicationCatalog の `ListMyApplications` が name 昇順で返す。割当アプリが
増えると固定順では目的のアプリを探しにくく、Okta End-User Dashboard も Entra ID
My Apps も「利用者による手動並び替え」と「管理者によるカテゴリ分類」で一覧を
整理できる。

並び替え機能には所有関係の判断が要る。手動順は (tenant, user, application) を
キーに持つ利用者個別の表示設定であり、これを IdentityManagement の User 集約に
持たせるか、ApplicationCatalog に持たせるかで結合先が変わる。

## 決定
1. **手動並び順は ApplicationCatalog が所有する**。
   per-user の手動順は application_id の順序列であり、Application を参照する表示設定
   である。User 集約に application_id の列を持たせると IdentityManagement が
   ApplicationCatalog の識別子へ結合してしまう。順序列は ApplicationCatalog 側の
   `ApplicationOrdering` (tenant_id + user_sub をキーに application_id の順序列を持つ
   entity) として永続化する。

2. **既定整列は計算で出す**。
   手動順が無いアプリは name 昇順で並べる。「最近利用」など利用頻度に基づく整列は
   利用イベントの集計を要するため本 ADR の対象外とし、初期は手動順 + name 昇順の
   2 段だけにする。

3. **手動順は割当の上に重ねる**。
   `ListMyApplications` は割当済み visible active アプリを解決した後、保存済み手動順を
   並び替えキーとして適用する。手動順に在るが現在は未割当のアプリは除外し、手動順に
   無い割当アプリは name 昇順で末尾に付ける。これにより新規割当・割当解除があっても
   一覧は破綻しない。

4. **手動並び替えはドメインイベントを emit しない**。
   個人の表示設定であり、認証・認可の wire 挙動や監査対象の状態遷移ではない。
   `ReorderMyApplications` は順序列を upsert するに留める。

5. **カテゴリ分類も ApplicationCatalog が所有する** (slice 2)。
   カテゴリは管理者が tenant 単位で定義し Application に 0..N 個割り当てる。定義は
   ApplicationCatalog の `ApplicationCategory` entity とし、管理操作は
   `AdminApplicationCategoriesManage` 権限で保護する。本決定は slice 2 で実装に反映する。

## 却下した代替案
- **手動順を IdentityManagement の User portal preference に置く**: User 集約が
  ApplicationCatalog の application_id へ結合し、コンテキスト境界を越える。表示設定の
  所有を表示対象のコンテキストへ寄せる本決定の方が結合が少ない。
- **手動順を ApplicationAssignment に持たせる**: 順序は per-user、割当は
  per-subject (user / group) で粒度が異なる。group 割当を共有する利用者間で順序が
  混ざるため不適。
- **既定整列に「最近利用」を初期から入れる**: 利用イベント集計の基盤を要し、
  並び替え機能のリリースを不必要に重くする。後続 WI で扱う。

## 影響
- ApplicationCatalog に `ApplicationOrdering` entity と `GetMyApplicationOrder` /
  `ReorderMyApplications` interface が加わる。
- `ListMyApplications` の戻り順が「手動順 → name 昇順」へ変わる (既定では手動順が
  空なので従来同様 name 昇順)。
- 新 DB table `application_orderings` (tenant 境界、user_sub キー)。
- slice 2 で `ApplicationCategory` entity・カテゴリ付与・ポータルのセクション表示と
  `AdminApplicationCategoriesManage` 権限が加わる。
