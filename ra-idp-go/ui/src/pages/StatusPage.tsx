import { Button, Stack, Text, ThemeIcon, Title } from "@mantine/core";
import {
  IconCheck,
  IconLogin,
  IconLogout,
  IconX,
} from "@tabler/icons-react";
import { AuthShell } from "../components/AuthShell";
import type { StatusPage as StatusPageData } from "../types";

const content = {
  approved: {
    title: "デバイスを承認しました",
    text: "この画面を閉じて、デバイスに戻ることができます。",
    icon: IconCheck,
    color: "teal",
  },
  denied: {
    title: "接続を拒否しました",
    text: "デバイスにはアカウント情報は共有されていません。",
    icon: IconX,
    color: "gray",
  },
  "signed-out": {
    title: "ログアウトしました",
    text: "セッションは安全に終了しました。",
    icon: IconLogout,
    color: "indigo",
  },
  "authentication-required": {
    title: "ログインが必要です",
    text: "デバイスを承認する前に、認証フローからログインしてください。",
    icon: IconLogin,
    color: "orange",
  },
} as const;

export function StatusPage({ status }: StatusPageData) {
  const state = content[status];
  const StatusIcon = state.icon;

  return (
    <AuthShell>
      <Stack gap="xl" align="center" py="xl">
        <ThemeIcon size={68} radius="xl" variant="light" color={state.color}>
          <StatusIcon size={34} />
        </ThemeIcon>
        <Stack gap="xs" align="center">
          <Title order={2} ta="center">{state.title}</Title>
          <Text c="dimmed" ta="center">{state.text}</Text>
        </Stack>
        <Button component="a" href="/.well-known/openid-configuration" variant="light">
          プロバイダー情報
        </Button>
      </Stack>
    </AuthShell>
  );
}
