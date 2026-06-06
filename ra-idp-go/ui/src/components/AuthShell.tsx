import {
  Anchor,
  Box,
  Container,
  Group,
  Paper,
  Stack,
  Text,
} from "@mantine/core";
import { IconLock } from "@tabler/icons-react";
import type { ReactNode } from "react";
import { Brand } from "./Brand";

type AuthShellProps = {
  children: ReactNode;
  asideTitle?: string;
  asideText?: string;
};

export function AuthShell({
  children,
  asideTitle = "ひとつの安全な入口から、すべてのサービスへ。",
  asideText = "標準準拠の認証基盤が、アカウントとアプリケーションを保護します。",
}: AuthShellProps) {
  return (
    <Box className="auth-background">
      <Container size={1060} className="auth-container">
        <Paper className="auth-frame" radius={28} shadow="xl">
          <Box className="auth-aside">
            <Brand />
            <Stack gap="lg" className="auth-aside-copy">
              <Text className="eyebrow">Regenerative Architecture</Text>
              <Text component="h1" className="aside-title">
                {asideTitle}
              </Text>
              <Text className="aside-text">{asideText}</Text>
            </Stack>
            <Group gap="xs" className="trust-note">
              <IconLock size={16} />
              <Text size="sm">OpenID Connect / OAuth 2.0</Text>
            </Group>
          </Box>

          <Box className="auth-main">
            <Box className="mobile-brand">
              <Brand />
            </Box>
            {children}
            <Group justify="center" gap="xs" mt="xl">
              <Anchor href="/.well-known/openid-configuration" size="xs" c="dimmed">
                Provider information
              </Anchor>
              <Text size="xs" c="gray.5">•</Text>
              <Text size="xs" c="dimmed">Privacy protected</Text>
            </Group>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
