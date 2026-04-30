import React from 'react';
import { Button, Tooltip } from '@douyinfe/semi-ui';
import { Sparkles } from 'lucide-react';

const FrontendSwitcher = ({ onSwitchFrontend, t }) => (
  <Tooltip content={t('使用新版现代化前端')} position='bottom'>
    <span className='inline-flex'>
      <Button
        icon={<Sparkles size={18} />}
        aria-label={t('使用新版现代化前端')}
        onClick={onSwitchFrontend}
        theme='borderless'
        type='tertiary'
        className='!p-1.5 !text-current focus:!bg-semi-color-fill-1 !rounded-full !bg-semi-color-fill-0 hover:!bg-semi-color-fill-1'
      />
    </span>
  </Tooltip>
);

export default FrontendSwitcher;
