import { Box, Group, Stack, Text, ThemeIcon } from "@mantine/core";
import { IconFingerprint } from "@tabler/icons-react";

export function Brand() {
  return (
    <Group gap="sm" wrap="nowrap">
      <ThemeIcon size={42} radius={12} variant="gradient" gradient={{ from: "indigo", to: "cyan" }}>
        <IconFingerprint size={25} stroke={1.8} />
      </ThemeIcon>
      <Stack gap={0}>
        <Text fw={750} size="lg" lh={1.2}>
          RA Identity
        </Text>
        <Text c="dimmed" size="xs" fw={600} tt="uppercase" lts="0.08em">
          Secure access
        </Text>
      </Stack>
      <Box />
    </Group>
  );
}
