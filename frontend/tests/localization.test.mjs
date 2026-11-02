import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { build } from 'esbuild';

const compiled = await build({
    entryPoints: ['src/utils/helpers.ts', 'src/utils/translations.ts'],
    bundle: true,
    write: false,
    outdir: 'tests',
    format: 'esm',
    platform: 'node',
    loader: { '.gif': 'dataurl', '.png': 'dataurl' },
});
const [helpers, translations] = await Promise.all(compiled.outputFiles.map(file =>
    import(`data:text/javascript;base64,${Buffer.from(file.text).toString('base64')}`)
));
const russian = JSON.parse(await readFile('../internal/localization/locales/ru-RU.json', 'utf8'));
const t = (key, fallback) => russian[key] || fallback;

test('Russian RUB prices use decimal commas and spaces without changing other combinations', () => {
    const state = { display_language: 'ru-RU', price_prefix: '', price_suffix: ' ₽', translations: russian };
    for (const [value, expected] of [[125.5, '125,50 ₽'], [1250, '1 250,00 ₽'], [12345.67, '12 345,67 ₽']]) {
        assert.equal(helpers.formatPrice(value, state), expected);
    }
    for (const language of ['en-US', 'tr-TR']) {
        assert.equal(helpers.formatPrice(125.5, { ...state, display_language: language }), '125.50 ₽');
    }
    assert.equal(helpers.formatPrice(125.5, { ...state, price_prefix: '$', price_suffix: '' }), '$125.50');
    assert.equal(helpers.formatPrice(125.5, { ...state, price_suffix: '€' }), '125.50€');
    assert.equal(helpers.formatPrice(undefined, state), 'Н/Д');
    assert.equal(helpers.formatPrice(Infinity, state), 'Н/Д');
});

test('Russian labels resolve from the catalog through existing translation helpers', () => {
    assert.equal(translations.translateRarity('RARE', t, 'ru-RU'), 'Редкий');
    assert.equal(translations.translateItemType('MATERIAL', t, 'ru-RU'), 'Материал');
    assert.equal(translations.translateGear('SWORD', t, 'ru-RU'), 'Меч');
    assert.equal(translations.bestSellReasonLabel('high_buy_orders', t, 'ru-RU'), 'Много заявок на покупку');
    assert.equal(helpers.formatLocation('inventory, stash', { translations: russian }), 'Инвентарь, Хранилище');
    assert.equal(helpers.formatRemainingDuration(3661, 'ru-RU', t), '1ч 1мин');
    assert.equal(helpers.formatRemainingDuration(65, 'ru-RU', t), '1мин 05с');
    assert.equal(helpers.formatPricingEtaText('Осталось %s', 5, 'ru-RU', t), 'Осталось 5с');
    assert.equal(helpers.formatRemainingDuration(65, 'tr-TR'), '1dk 05sn');
    assert.equal(helpers.formatRemainingDuration(65, 'en-US'), '1m 05s');
});
