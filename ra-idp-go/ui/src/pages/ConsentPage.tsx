import {
  Avatar,
  Button,
  Group,
  Paper,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {
  IconArrowRight,
  IconCheck,
  IconId,
  IconMail,
  IconShieldCheck,
  IconUser,
  IconX,
} from "@tabler/icons-react";
import { AuthShell } from "../components/AuthShell";
import type { ConsentPage as ConsentPageData } from "../types";

const scopeDetails: Record<string, { label: string; description: string; icon: typeof IconId }> = {
  openid: { label: "本人確認", description: "一意のアカウントIDを確認します", icon: IconId },
  profile: { label: "基本プロフィール", description: "名前とプロフィール情報を参照します", icon: IconUser },
  email: { label: "メールアドレス", description: "登録済みメールアドレスを参照します", icon: IconMail },
};

export function ConsentPage({ requestId, clientName, scope }: ConsentPageData) {
  const scopes = scope.split(/\s+/).filter(Boolean);

  return (
    <AuthShell
      asideTitle="共有する情報は、いつでもあなたが決められます。"
      asideText="アプリケーションには、許可した情報だけが安全に共有されます。"
    >
      <Stack gap="lg">
        <Stack align="center" gap="sm">
          <Avatar size={58} radius={16} color="indigo" variant="light">
            {clientName.slice(0, 2).toUpperCase()}
          </Avatar>
          <Stack gap={4} align="center">
            <Title order={2} ta="center">{clientName}</Title>
            <Text c="dimmed" ta="center">このアプリがアカウントへのアクセスを求めています</Text>
          </Stack>
        </Stack>

        <Paper withBorder radius="lg" p="md">
          <Stack gap="md">
            <Group gap="sm">
              <ThemeIcon variant="light" color="teal" radius="xl">
                <IconShieldCheck size={18} />
              </ThemeIcon>
              <Text fw={650}>許可する内容</Text>
            </Group>
            {scopes.map((scopeName) => {
              const detail = scopeDetails[scopeName] ?? {
                label: scopeName,
                description: "このアプリが要求する追加の権限です",
                icon: IconCheck,
              };
              const ScopeIcon = detail.icon;
              return (
                <Group key={scopeName} wrap="nowrap" align="flex-start">
                  <ThemeIcon variant="subtle" color="gray">
                    <ScopeIcon size={19} />
                  </ThemeIcon>
                  <Stack gap={1}>
                    <Text size="sm" fw={600}>{detail.label}</Text>
                    <Text size="xs" c="dimmed">{detail.description}</Text>
                  </Stack>
                </Group>
              );
            })}
          </Stack>
        </Paper>

        <form method="POST" action="/consent">
          <input type="hidden" name="request_id" value={requestId} />
          <Stack gap="sm">
            <Button type="submit" name="action" value="allow" size="md" rightSection={<IconArrowRight size={18} />}>
              アクセスを許可
            </Button>
            <Button type="submit" name="action" value="deny" size="md" variant="subtle" color="gray" leftSection={<IconX size={17} />}>
              キャンセル
            </Button>
          </Stack>
        </form>
      </Stack>
    </AuthShell>
  );
}
