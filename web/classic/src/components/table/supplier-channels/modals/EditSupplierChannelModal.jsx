/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useRef, useState } from 'react';
import {
  Button,
  SideSheet,
  Space,
  Spin,
  Typography,
  Card,
  Tag,
  Avatar,
  Form,
  Row,
  Col,
} from '@douyinfe/semi-ui';
import { IconSave, IconClose, IconServer, IconCreditCard } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { showError, selectFilter, getChannelIcon } from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import { CHANNEL_OPTIONS } from '../../../../constants';

const { Text, Title } = Typography;

// Normalize a string of comma/newline separated values into a trimmed, de-duped array
const splitToArray = (value) => {
  if (!value) return [];
  if (Array.isArray(value)) {
    return Array.from(new Set(value.map((v) => (v || '').trim()).filter(Boolean)));
  }
  return Array.from(
    new Set(
      String(value)
        .split(/[,\n]/)
        .map((v) => v.trim())
        .filter(Boolean),
    ),
  );
};

const EditSupplierChannelModal = (props) => {
  const { visible, editingChannel, handleClose, getChannel, createChannel, updateChannel } =
    props;
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const isEdit = editingChannel?.id !== undefined;

  // 渲染带 logo 的类型下拉项，与管理员端创建渠道体验对齐
  const renderTypeOption = (renderProps) => {
    const {
      disabled,
      selected,
      label,
      value,
      focused,
      className,
      style,
      onMouseEnter,
      onClick,
    } = renderProps;

    const optionClassName = [
      'flex items-center gap-3 px-3 py-2 transition-all duration-200 rounded-lg mx-2 my-1',
      focused && 'bg-blue-50 shadow-sm',
      selected &&
        'bg-blue-100 text-blue-700 shadow-lg ring-2 ring-blue-200 ring-opacity-50',
      disabled && 'opacity-50 cursor-not-allowed',
      !disabled && 'hover:bg-gray-50 hover:shadow-md cursor-pointer',
      className,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <div
        style={style}
        className={optionClassName}
        onClick={() => !disabled && onClick()}
        onMouseEnter={() => onMouseEnter()}
      >
        <div className='flex items-center gap-3 w-full'>
          <div className='flex-shrink-0 w-5 h-5 flex items-center justify-center'>
            {getChannelIcon(value)}
          </div>
          <div className='flex-1 min-w-0'>
            <span className='text-sm font-medium truncate'>{label}</span>
          </div>
          {selected && (
            <div className='flex-shrink-0 text-blue-600'>
              <svg width='16' height='16' viewBox='0 0 16 16' fill='currentColor'>
                <path d='M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z' />
              </svg>
            </div>
          )}
        </div>
      </div>
    );
  };

  const getInitValues = () => ({
    name: '',
    type: 1,
    key: '',
    base_url: '',
    models: [],
    groups: [],
    cost_price: undefined,
  });

  // Load full channel (with key) for editing
  const loadChannel = async () => {
    setLoading(true);
    try {
      const data = await getChannel(editingChannel.id);
      if (data) {
        const values = {
          name: data.name || '',
          type: data.type ?? 1,
          // do not prefill key on edit; leave blank to keep existing
          key: '',
          base_url: data.base_url || '',
          models: splitToArray(data.models),
          groups: splitToArray(data.group),
          cost_price: data.cost_price ?? undefined,
        };
        formApiRef.current?.setValues(values);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      if (isEdit) {
        loadChannel();
      } else {
        formApiRef.current?.setValues(getInitValues());
      }
    } else {
      formApiRef.current?.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, editingChannel?.id]);

  const handleCancel = () => {
    handleClose();
  };

  const submit = async (values) => {
    // Client-side validation
    const costPrice = Number(values.cost_price);
    if (!Number.isFinite(costPrice) || costPrice <= 0) {
      showError(t('成本价必须大于 0'));
      return;
    }
    const models = splitToArray(values.models);
    const groups = splitToArray(values.groups);
    if (!isEdit) {
      if (!values.name || !values.name.trim()) {
        showError(t('请输入名称'));
        return;
      }
      if (!values.key || !values.key.trim()) {
        showError(t('请输入密钥'));
        return;
      }
      if (models.length === 0) {
        showError(t('请至少填写一个模型'));
        return;
      }
    }

    // Build payload matching the backend channel object
    const payload = {
      name: (values.name || '').trim(),
      type: values.type,
      base_url: (values.base_url || '').trim(),
      models: models.join(','),
      group: groups.length ? groups.join(',') : 'default',
      cost_price: costPrice,
    };
    // Only send key when provided (on edit, blank means keep existing)
    if (values.key && values.key.trim()) {
      payload.key = values.key.trim();
    }

    setLoading(true);
    try {
      let ok = false;
      if (isEdit) {
        ok = await updateChannel({ ...payload, id: parseInt(editingChannel.id, 10) });
      } else {
        ok = await createChannel(payload);
      }
      if (ok) {
        handleClose();
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <SideSheet
      placement={isEdit ? 'right' : 'left'}
      title={
        <Space>
          {isEdit ? (
            <Tag color='blue' shape='circle'>
              {t('更新')}
            </Tag>
          ) : (
            <Tag color='green' shape='circle'>
              {t('新建')}
            </Tag>
          )}
          <Title heading={4} className='m-0'>
            {isEdit ? t('更新渠道信息') : t('新增渠道')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: '0' }}
      visible={visible}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              className='!rounded-lg'
              type='primary'
              onClick={handleCancel}
              icon={<IconClose />}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
      closeIcon={null}
      onCancel={handleCancel}
    >
      <Spin spinning={loading}>
        <Form
          key={isEdit ? 'edit' : 'new'}
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          <div className='p-2'>
            {/* 基本信息 */}
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                  <IconServer size={16} />
                </Avatar>
                <div>
                  <Text className='text-lg font-medium'>{t('基本信息')}</Text>
                  <div className='text-xs text-gray-600'>
                    {t('设置渠道的基本信息')}
                  </div>
                </div>
              </div>
              <Row gutter={12}>
                <Col span={24}>
                  <Form.Input
                    field='name'
                    label={t('名称')}
                    placeholder={t('请输入名称')}
                    rules={[{ required: true, message: t('请输入名称') }]}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Select
                    field='type'
                    label={t('类型')}
                    placeholder={t('请选择类型')}
                    optionList={CHANNEL_OPTIONS}
                    filter={selectFilter}
                    renderOptionItem={renderTypeOption}
                    rules={[{ required: true, message: t('请选择类型') }]}
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col span={24}>
                  <Form.Input
                    field='key'
                    label={t('密钥')}
                    mode='password'
                    placeholder={
                      isEdit
                        ? t('如不修改请留空')
                        : t('请输入密钥')
                    }
                    rules={
                      isEdit
                        ? []
                        : [{ required: true, message: t('请输入密钥') }]
                    }
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Input
                    field='base_url'
                    label={t('API 地址')}
                    placeholder={t('选填，填写自定义 API 地址；留空则直连官方')}
                    extraText={t('填写后该渠道将被标记为「中转」，留空表示官方直连官key')}
                    showClear
                  />
                </Col>
                <Col span={24}>
                  <Form.Select
                    field='models'
                    label={t('模型')}
                    placeholder={t('请输入模型名称，回车确认，可输入多个')}
                    multiple
                    allowCreate
                    filter
                    defaultActiveFirstOption={false}
                    rules={
                      isEdit
                        ? []
                        : [
                            {
                              required: true,
                              type: 'array',
                              min: 1,
                              message: t('请至少填写一个模型'),
                            },
                          ]
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col span={24}>
                  <Form.Select
                    field='groups'
                    label={t('分组')}
                    placeholder={t('请选择或输入分组')}
                    multiple
                    allowCreate
                    filter
                    defaultActiveFirstOption={false}
                    rules={[
                      {
                        required: true,
                        type: 'array',
                        min: 1,
                        message: t('请至少选择一个分组'),
                      },
                    ]}
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>
            </Card>

            {/* 计费设置 */}
            <Card className='!rounded-2xl shadow-sm border-0'>
              <div className='flex items-center mb-2'>
                <Avatar size='small' color='green' className='mr-2 shadow-md'>
                  <IconCreditCard size={16} />
                </Avatar>
                <div>
                  <Text className='text-lg font-medium'>{t('计费设置')}</Text>
                  <div className='text-xs text-gray-600'>
                    {t('设置该渠道的成本价')}
                  </div>
                </div>
              </div>
              <Row gutter={12}>
                <Col span={24}>
                  <Form.InputNumber
                    field='cost_price'
                    label={t('成本价')}
                    prefix='¥'
                    placeholder={t('请输入成本价')}
                    min={0}
                    step={0.1}
                    extraText={t('成本价必须大于 0')}
                    rules={[
                      { required: true, message: t('请输入成本价') },
                      {
                        validator: (rule, value) => {
                          if (value === undefined || value === null || value === '') {
                            return Promise.resolve();
                          }
                          return Number(value) > 0
                            ? Promise.resolve()
                            : Promise.reject(t('成本价必须大于 0'));
                        },
                      },
                    ]}
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>
            </Card>
          </div>
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default EditSupplierChannelModal;
