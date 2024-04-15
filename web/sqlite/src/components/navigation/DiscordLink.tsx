import styles from './DiscordLink.module.scss';
import { Button } from 'react-bulma-components';
import * as React from 'react';

export type DiscordLinkProps = {
  guildID: string,
  channelID?: string,
  messageID?: string,
} & React.PropsWithChildren;

function DiscordLink({
  guildID,
  channelID,
  messageID,
  children,
}: DiscordLinkProps) {
  // Could also use Discord Canary (https://canary.discord.com/channels/...) but
  // I don't bother supporting that here.
  const url = `https://discord.com/channels/${[guildID, channelID, messageID]
    .filter(Number)
    .map(v => v?.toString() || '')
    .map(v => encodeURIComponent(v))
    .join('/')}`;

  return (
    <Button
      renderAs='a'
      className={styles.discordLink}
      href={url}>
      {children}
    </Button>
  );
}

export default DiscordLink;
