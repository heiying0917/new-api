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
import { useTranslation } from 'react-i18next';
import {
  Avatar,
  Button,
  Card,
  Col,
  Form,
  Row,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconClose,
  IconSave,
  IconUser,
} from '@douyinfe/semi-icons';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const EditSupplierModal = (props) => {
  const { t } = useTranslation();
  const { visible, handleClose, refresh, updateSupplier, editingSupplier } =
    props;
  const isMobile = useIsMobile();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);

  const userId = editingSupplier?.user_id;

  const getInitValues = () => ({
    priority: editingSupplier?.priority ?? 0,
    enabled: editingSupplier?.enabled ?? false,
    settlement_mode: editingSupplier?.settlement_mode ?? 'manual',
    settlement_cycle: editingSupplier?.settlement_cycle ?? 'month',
    remark: editingSupplier?.remark ?? '',
  });

  useEffect(() => {
    if (visible && formApiRef.current) {
      formApiRef.current.setValues(getInitValues());
    }
  }, [visible, editingSupplier?.user_id]);

  const handleCancel = () => handleClose();

  const submit = async (values) => {
    if (userId === undefined || userId === null) {
      return;
    }
    setLoading(true);
    const payload = {
      user_id: userId,
      priority: values.priority,
      enabled: values.enabled,
      settlement_mode: values.settlement_mode,
      settlement_cycle: values.settlement_cycle,
      remark: values.remark,
    };
    const success = await updateSupplier(payload);
    setLoading(false);
    if (success) {
      await refresh();
      handleClose();
    }
  };

  return (
    <SideSheet
      placement='right'
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('编辑')}
          </Tag>
          <Title heading={4} className='m-0'>
            {t('编辑供应商')}
          </Title>
        </Space>
      }
      bodyStyle={{ padding: 0 }}
      visible={visible}
      width={isMobile ? '100%' : 600}
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              onClick={() => formApiRef.current?.submitForm()}
              icon={<IconSave />}
              loading={loading}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
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
          initValues={getInitValues()}
          getFormApi={(api) => (formApiRef.current = api)}
          onSubmit={submit}
        >
          {() => (
            <div className='p-2 space-y-3'>
              <Card className='!rounded-2xl shadow-sm border-0'>
                <div className='flex items-center mb-2'>
                  <Avatar size='small' color='blue' className='mr-2 shadow-md'>
                    <IconUser size={16} />
                  </Avatar>
                  <div>
                    <Text className='text-lg font-medium'>
                      {t('供应商设置')}
                    </Text>
                    <div className='text-xs text-gray-600'>
                      {editingSupplier?.username
                        ? `${t('用户名')}: ${editingSupplier.username}`
                        : t('供应商结算与优先级配置')}
                    </div>
                  </div>
                </div>

                <Row gutter={12}>
                  <Col span={24}>
                    <Form.InputNumber
                      field='priority'
                      label={t('优先级')}
                      placeholder={t('请输入优先级')}
                      style={{ width: '100%' }}
                    />
                  </Col>

                  <Col span={24}>
                    <Form.Switch
                      field='enabled'
                      label={t('启用')}
                      checkedText={t('开')}
                      uncheckedText={t('关')}
                    />
                  </Col>

                  <Col span={24}>
                    <Form.Select
                      field='settlement_mode'
                      label={t('结算方式')}
                      placeholder={t('请选择结算方式')}
                      style={{ width: '100%' }}
                      optionList={[
                        { label: t('手动结算'), value: 'manual' },
                        { label: t('自动结算'), value: 'auto' },
                      ]}
                    />
                  </Col>

                  <Col span={24}>
                    <Form.Select
                      field='settlement_cycle'
                      label={t('结算周期')}
                      placeholder={t('请选择结算周期')}
                      style={{ width: '100%' }}
                      optionList={[
                        { label: t('按天'), value: 'day' },
                        { label: t('按周'), value: 'week' },
                        { label: t('按月'), value: 'month' },
                      ]}
                    />
                  </Col>

                  <Col span={24}>
                    <Form.TextArea
                      field='remark'
                      label={t('备注')}
                      placeholder={t('请输入备注（仅管理员可见）')}
                      autosize
                      rows={2}
                    />
                  </Col>
                </Row>
              </Card>
            </div>
          )}
        </Form>
      </Spin>
    </SideSheet>
  );
};

export default EditSupplierModal;
