// Package ports は authentication ユースケースが外界に求める境界を定義する。
//
// Password storage の境界。実装は Argon2id 等の OWASP 推奨アルゴリズムを用い、
// 戻り値の encoded 文字列にアルゴリズム・パラメータ・salt を内包する PHC 形式
// を期待する。
package ports

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}
