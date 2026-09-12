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

export const PRICING_SORT_OPTIONS = {
  NAME: 'name',
  PRICE_LOW: 'price-low',
  PRICE_HIGH: 'price-high',
  LISTED_NEWEST: 'listed-newest',
  LISTED_OLDEST: 'listed-oldest',
};

export const PRICING_QUOTA_SORT_OPTIONS = {
  ASC: 'quota-asc',
  DESC: 'quota-desc',
};

export function getPricingSortOptions(t) {
  return [
    { value: PRICING_SORT_OPTIONS.NAME, label: t('名称') },
    { value: PRICING_SORT_OPTIONS.PRICE_LOW, label: t('价格从低到高') },
    { value: PRICING_SORT_OPTIONS.PRICE_HIGH, label: t('价格从高到低') },
    { value: PRICING_SORT_OPTIONS.LISTED_NEWEST, label: t('最新上架') },
    { value: PRICING_SORT_OPTIONS.LISTED_OLDEST, label: t('最早上架') },
  ];
}

function compareNames(a, b) {
  return (a.model_name || '').localeCompare(b.model_name || '');
}

function getModelPrice(model) {
  return model.quota_type === 0 ? model.model_ratio : model.model_price || 0;
}

function getListingTime(model) {
  return Number.isFinite(model.created_time) && model.created_time > 0
    ? model.created_time
    : 0;
}

function compareListingTimes(a, b, direction) {
  const aTime = getListingTime(a);
  const bTime = getListingTime(b);
  // Unknown dates stay last in both directions, including with older backends.
  if (!aTime && bTime) return 1;
  if (aTime && !bTime) return -1;
  return direction * (aTime - bTime) || compareNames(a, b);
}

export function sortPricingModels(models, sortBy) {
  const sorted = [...models];
  switch (sortBy) {
    case PRICING_SORT_OPTIONS.PRICE_LOW:
      return sorted.sort((a, b) => getModelPrice(a) - getModelPrice(b));
    case PRICING_SORT_OPTIONS.PRICE_HIGH:
      return sorted.sort((a, b) => getModelPrice(b) - getModelPrice(a));
    case PRICING_SORT_OPTIONS.LISTED_NEWEST:
      return sorted.sort((a, b) => compareListingTimes(a, b, -1));
    case PRICING_SORT_OPTIONS.LISTED_OLDEST:
      return sorted.sort((a, b) => compareListingTimes(a, b, 1));
    default:
      return sorted.sort(compareNames);
  }
}
