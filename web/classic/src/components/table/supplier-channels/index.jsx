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

import React, { useCallback } from 'react';
import { Button, Form, Typography } from '@douyinfe/semi-ui';
import { IconSearch, IconPlus } from '@douyinfe/semi-icons';
import CardPro from '../../common/ui/CardPro';
import SupplierChannelsTable from './SupplierChannelsTable';
import EditSupplierChannelModal from './modals/EditSupplierChannelModal';
import { useSupplierChannelsData } from '../../../hooks/supplier-channels/useSupplierChannelsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const { Title } = Typography;

function SupplierChannelsPage() {
  const data = useSupplierChannelsData();
  const isMobile = useIsMobile();
  const {
    channels,
    loading,
    activePage,
    pageSize,
    total,
    showEdit,
    setShowEdit,
    editingChannel,
    setEditingChannel,
    closeEdit,
    searchChannels,
    getChannel,
    createChannel,
    updateChannel,
    deleteChannel,
    handlePageChange,
    handlePageSizeChange,
    t,
  } = data;

  const handleAdd = () => {
    setEditingChannel({ id: undefined });
    setShowEdit(true);
  };

  const handleEdit = useCallback(
    (record) => {
      setEditingChannel(record);
      setShowEdit(true);
    },
    [setEditingChannel, setShowEdit],
  );

  const handleDelete = useCallback(
    (id) => {
      deleteChannel(id);
    },
    [deleteChannel],
  );

  const handleSearch = (values) => {
    searchChannels(values?.keyword || '');
  };

  return (
    <>
      <EditSupplierChannelModal
        visible={showEdit}
        editingChannel={editingChannel}
        handleClose={closeEdit}
        getChannel={getChannel}
        createChannel={createChannel}
        updateChannel={updateChannel}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex items-center'>
            <Title heading={5} className='m-0'>
              {t('我的渠道')}
            </Title>
          </div>
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <div className='flex flex-wrap gap-2 w-full md:w-auto order-2 md:order-1'>
              <Button
                type='primary'
                className='flex-1 md:flex-initial'
                icon={<IconPlus />}
                onClick={handleAdd}
                size='small'
              >
                {t('新增渠道')}
              </Button>
            </div>

            <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
              <Form
                onSubmit={handleSearch}
                allowEmpty={true}
                autoComplete='off'
                layout='horizontal'
                trigger='change'
                className='w-full md:w-auto'
              >
                <div className='flex flex-col md:flex-row items-center gap-2 w-full md:w-auto'>
                  <div className='relative w-full md:w-56'>
                    <Form.Input
                      field='keyword'
                      prefix={<IconSearch />}
                      placeholder={t('搜索渠道名称')}
                      showClear
                      pure
                      size='small'
                    />
                  </div>
                  <Button
                    type='tertiary'
                    htmlType='submit'
                    loading={loading}
                    className='w-full md:w-auto'
                    size='small'
                  >
                    {t('查询')}
                  </Button>
                </div>
              </Form>
            </div>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize: pageSize,
          total: total,
          onPageChange: handlePageChange,
          onPageSizeChange: handlePageSizeChange,
          isMobile: isMobile,
          t: t,
        })}
        t={t}
      >
        <SupplierChannelsTable
          channels={channels}
          loading={loading}
          activePage={activePage}
          pageSize={pageSize}
          total={total}
          handlePageChange={handlePageChange}
          handlePageSizeChange={handlePageSizeChange}
          onEdit={handleEdit}
          onDelete={handleDelete}
          t={t}
        />
      </CardPro>
    </>
  );
}

export default SupplierChannelsPage;
