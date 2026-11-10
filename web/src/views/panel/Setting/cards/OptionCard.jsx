import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';

import { FormCard } from '@/components/settings/cards';
import { useOptionCard } from '../parts';
import { cardKeys } from '../registry';

// ==============================|| PLATFORM SETTINGS — OPTION FORM CARD ||============================== //
// A FormCard whose save scope is the card's option keys from the registry. Title, description and
// footer hint come from `settingsUi.platform.cards.<id>`; `enableKey` puts that boolean option in
// the header as the card's on/off switch (saved with the footer like the rest).

export const cardText = (t, id, part) => t(`settingsUi.platform.cards.${id}.${part}`, { defaultValue: '' }) || undefined;

// Switch props for a boolean option.
export const bindSwitch = (ctx, key) => ({ checked: ctx.inputs[key] === 'true', onCheckedChange: (v) => ctx.setBool(key, v) });

export default function OptionCard({ ctx, id, options, enableKey, enableLabel, title, description, actions, children }) {
  const { t } = useTranslation();
  const { cardProps } = useOptionCard(ctx, cardKeys(id), options);

  return (
    <FormCard
      title={title ?? cardText(t, id, 'title')}
      description={description ?? cardText(t, id, 'description')}
      hint={cardText(t, id, 'hint')}
      highlight={id}
      enable={enableKey ? { id: enableKey, label: enableLabel, ...bindSwitch(ctx, enableKey) } : undefined}
      actions={actions}
      {...cardProps}
    >
      {children}
    </FormCard>
  );
}

OptionCard.propTypes = {
  ctx: PropTypes.object.isRequired,
  id: PropTypes.string.isRequired,
  options: PropTypes.object,
  enableKey: PropTypes.string,
  enableLabel: PropTypes.node,
  title: PropTypes.node,
  description: PropTypes.node,
  actions: PropTypes.node,
  children: PropTypes.node
};
