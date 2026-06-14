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
import { Modal, Form } from '@douyinfe/semi-ui';

const toNumber = (v) => {
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
};

const ConfirmModal = ({ visible, record, onCancel, confirmSettlement, t }) => {
  const formApiRef = useRef(null);
  const [submitting, setSubmitting] = useState(false);

  const initValues = {
    actual_amount: record ? toNumber(record.computed_cny) : 0,
    actual_currency: 'CNY',
    settle_method: '',
    remark: '',
  };

  // Reset form values whenever the modal opens for a record
  useEffect(() => {
    if (visible && formApiRef.current && record) {
      formApiRef.current.setValues({
        actual_amount: toNumber(record.computed_cny),
        actual_currency: 'CNY',
        settle_method: '',
        remark: '',
      });
    }
  }, [visible, record]);

  const handleOk = async () => {
    if (!record || !formApiRef.current) return;
    try {
      const values = await formApiRef.current.validate();
      setSubmitting(true);
      const ok = await confirmSettlement(record.id, {
        actual_amount: toNumber(values.actual_amount),
        actual_currency: values.actual_currency || 'CNY',
        settle_method: values.settle_method || '',
        remark: values.remark || '',
      });
      setSubmitting(false);
      if (ok) {
        onCancel();
      }
    } catch (e) {
      // validation failed, keep modal open
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={t('确认结算')}
      visible={visible}
      onCancel={onCancel}
      onOk={handleOk}
      confirmLoading={submitting}
      maskClosable={false}
    >
      <Form
        initValues={initValues}
        getFormApi={(api) => {
          formApiRef.current = api;
        }}
      >
        <Form.InputNumber
          field='actual_amount'
          label={t('实付金额')}
          min={0}
          step={0.01}
          precision={2}
          style={{ width: '100%' }}
          rules={[{ required: true, message: t('请输入实付金额') }]}
        />
        <Form.Select
          field='actual_currency'
          label={t('结算币种')}
          style={{ width: '100%' }}
          optionList={[
            { label: t('人民币 (CNY)'), value: 'CNY' },
            { label: t('美元 (USD)'), value: 'USD' },
          ]}
          rules={[{ required: true, message: t('请选择结算币种') }]}
        />
        <Form.Input
          field='settle_method'
          label={t('结算方式')}
          placeholder={t('例如：支付宝 / 银行转账')}
        />
        <Form.TextArea
          field='remark'
          label={t('备注')}
          placeholder={t('选填')}
          autosize={{ minRows: 2, maxRows: 5 }}
        />
      </Form>
    </Modal>
  );
};

export default ConfirmModal;
