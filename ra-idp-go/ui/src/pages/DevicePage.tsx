import {
  Button,
  Code,
  Group,
  PinInput,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import { IconDeviceDesktopCheck, IconShieldCheck, IconX } from "@tabler/icons-react";
import { useState } from "react";
import { AuthShell } from "../components/AuthShell";
import type { DevicePage as DevicePageData } from "../types";

export function DevicePage({ userCode }: DevicePageData) {
  const normalizedCode = userCode.replace(/-/g, "").toUpperCase();
  const [code, setCode] = useState(normalizedCode);

  return (
    <AuthShell
      asideTitle="新しいデバイスを、確かな手順で接続。"
      asideText="画面に表示されたコードを確認し、信頼できるデバイスだけを承認してください。"
    >
      <Stack gap="xl">
        <Stack align="center" gap="sm">
          <ThemeIcon size={58} radius={18} variant="light">
            <IconDeviceDesktopCheck size={30} />
          </ThemeIcon>
          <Title order={2} ta="center">デバイスを接続</Title>
          <Text c="dimmed" ta="center">
            接続先に表示されているコードを入力してください。
          </Text>
        </Stack>

        <form method="POST" action="/device">
          <input type="hidden" name="user_code" value={code} />
          <Stack gap="lg" align="center">
            <PinInput
              length={8}
              value={code}
              onChange={(value) => setCode(value.toUpperCase())}
              type="alphanumeric"
              size="md"
              gap={6}
              aria-label="ユーザーコード"
            />
            <Text size="xs" c="dimmed">
              例: <Code>ABCD-EFGH</Code>
            </Text>
            <Stack gap="sm" w="100%">
              <Button
                type="submit"
                name="action"
                value="allow"
                size="md"
                leftSection={<IconShieldCheck size={18} />}
                disabled={code.length !== 8}
              >
                デバイスを承認
              </Button>
              <Button
                type="submit"
                name="action"
                value="deny"
                size="md"
                variant="subtle"
                color="gray"
                leftSection={<IconX size={17} />}
                disabled={code.length !== 8}
              >
                拒否
              </Button>
            </Stack>
          </Stack>
        </form>

        <Group gap="xs" justify="center">
          <IconShieldCheck size={15} color="var(--mantine-color-teal-6)" />
          <Text size="xs" c="dimmed">心当たりのない接続は承認しないでください</Text>
        </Group>
      </Stack>
    </AuthShell>
  );
}
