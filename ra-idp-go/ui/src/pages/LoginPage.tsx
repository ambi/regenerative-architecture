import {
  Alert,
  Button,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { IconAlertCircle, IconArrowRight, IconAt, IconLock } from "@tabler/icons-react";
import { AuthShell } from "../components/AuthShell";
import type { LoginPage as LoginPageData } from "../types";

export function LoginPage({ requestId, error }: LoginPageData) {
  return (
    <AuthShell>
      <Stack gap="xl">
        <Stack gap={8}>
          <Text className="eyebrow">Welcome back</Text>
          <Title order={2}>アカウントにログイン</Title>
          <Text c="dimmed">続行するには認証情報を入力してください。</Text>
        </Stack>

        {error ? (
          <Alert icon={<IconAlertCircle size={18} />} color="red" variant="light" title="ログインできません">
            {error}
          </Alert>
        ) : null}

        <form method="POST" action="/login">
          <input type="hidden" name="request_id" value={requestId} />
          <Stack gap="md">
            <TextInput
              name="username"
              label="ユーザー名"
              placeholder="your.name"
              leftSection={<IconAt size={18} />}
              autoComplete="username"
              required
              autoFocus
              size="md"
            />
            <PasswordInput
              name="password"
              label="パスワード"
              placeholder="パスワードを入力"
              leftSection={<IconLock size={18} />}
              autoComplete="current-password"
              required
              size="md"
            />
            <Button type="submit" size="md" mt="sm" rightSection={<IconArrowRight size={18} />}>
              ログイン
            </Button>
          </Stack>
        </form>
      </Stack>
    </AuthShell>
  );
}
