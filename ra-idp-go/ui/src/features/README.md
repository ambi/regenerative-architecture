# UI features

`features/` は RA/SCL の feature に合わせた UI の文脈境界である。
画面、局所部品、局所ヘルパーは、横断部品でない限り該当 feature 配下に置く。

`slices/` という別名は使わない。Go の `slices` パッケージと紛れ、AI が読むべき
範囲を判断しにくくなるため。
