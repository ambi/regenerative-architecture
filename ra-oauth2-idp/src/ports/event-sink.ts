/**
 * Layer 3 — Application Logic (ポート定義)
 *
 * ドメインイベントの出力先。
 *
 * 現在は main.ts のクロージャ `emit` として存在していたが、
 * ADR-016 で「outbox → Kafka」配送を採用したため、
 * これを正式なポートに昇格させる。
 *
 * 実装側 (adapters/event-sink/):
 *  - ConsoleEventSink     ローカル開発・テスト用。stdout に JSON 行で出力。
 *  - OutboxEventSink      Postgres outbox テーブルに INSERT。usecase の DB トランザクションに参加。
 *  - KafkaDirectEventSink (デバッグ用) Kafka に直接 publish。dual-write 問題があるので本番は非推奨。
 *
 * 本番は OutboxEventSink + event-relay プロセス (infra/event-relay) の組み合わせを推奨。
 *
 * トピック割り当ては infra/event-routing.yaml の event_to_topic を権威とする。
 */

import type { DomainEvent } from '../spec-bindings/schemas'
import type { TransactionContext } from './transaction'

export interface EventSink {
  /**
   * イベントを永続化または配送する。
   *
   * @param event ドメインイベント (events.schema.json の oneOf)
   * @param tx 進行中のトランザクションコンテキスト。
   *           OutboxEventSink ではここに INSERT する。
   *           ConsoleEventSink は無視する。
   *           未指定の場合、実装は独立したトランザクション/接続で配送する。
   */
  publish(event: DomainEvent, tx?: TransactionContext): Promise<void>

  /**
   * バッチ配送 (任意)。
   * 効率化のため複数イベントを 1 ラウンドトリップで配送する。
   * デフォルト実装は publish のループでよい。
   */
  publishMany?(events: DomainEvent[], tx?: TransactionContext): Promise<void>

  /**
   * シャットダウン時に呼ばれる。
   * Kafka producer の flush などに使う。
   */
  close?(): Promise<void>
}
