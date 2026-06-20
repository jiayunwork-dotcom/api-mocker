<template>
  <div class="field-row" :style="{ paddingLeft: depth * 24 + 'px' }">
    <div class="field-controls">
      <el-input v-model="field.name" size="small" style="width:140px" placeholder="字段名" />
      <el-select v-model="field.type" size="small" style="width:110px" @change="onTypeChange">
        <el-option label="string" value="string" />
        <el-option label="number" value="number" />
        <el-option label="integer" value="integer" />
        <el-option label="boolean" value="boolean" />
        <el-option label="array" value="array" />
        <el-option label="object" value="object" />
      </el-select>
      <el-checkbox v-model="field.required" size="small">必填</el-checkbox>
      <el-input v-model="field.desc" size="small" style="width:120px" placeholder="说明" />

      <template v-if="field.type === 'string'">
        <el-input v-model="field.example" size="small" style="width:120px" placeholder="示例值" />
        <el-button size="small" text @click="showEnumEditor = !showEnumEditor">
          {{ field.enum && field.enum.length ? '枚举(' + field.enum.length + ')' : '枚举' }}
        </el-button>
      </template>

      <template v-if="field.type === 'number' || field.type === 'integer'">
        <el-input v-model="field.example" size="small" style="width:120px" placeholder="示例值" />
      </template>

      <template v-if="field.type === 'array' || field.type === 'object'">
        <el-button size="small" type="primary" text @click="$emit('add-child', index)">+ 子字段</el-button>
      </template>

      <el-select v-model="field.ref" size="small" style="width:130px" clearable placeholder="引用模型" v-if="field.type === 'object' || !field.type">
        <el-option v-for="m in models" :key="m.id" :label="m.name" :value="m.name" />
      </el-select>

      <el-button size="small" type="danger" text @click="$emit('remove', index)">删除</el-button>
    </div>

    <div v-if="showEnumEditor" class="enum-editor">
      <el-tag v-for="(e, i) in field.enum" :key="i" closable @close="field.enum.splice(i, 1)" style="margin-right:4px">
        {{ e }}
      </el-tag>
      <el-input
        size="small"
        style="width:120px"
        placeholder="添加枚举值"
        @keyup.enter="addEnumValue"
        v-model="newEnumVal"
      />
    </div>

    <div v-if="(field.type === 'object' || field.type === 'array') && field.children && field.children.length" class="children">
      <BodyFieldEditor
        v-for="(child, ci) in field.children"
        :key="ci"
        :field="child"
        :models="models"
        :depth="depth + 1"
        :index="ci"
        @add-child="$emit('add-child', { parentIndex: index, childIndex: $event })"
        @remove="$emit('remove', { parentIndex: index, childIndex: $event })"
      />
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  field: { type: Object, required: true },
  models: { type: Array, default: () => [] },
  depth: { type: Number, default: 0 },
  index: { type: Number, default: 0 }
})

const emit = defineEmits(['add-child', 'remove'])

const showEnumEditor = ref(false)
const newEnumVal = ref('')

function onTypeChange(type) {
  if ((type === 'object' || type === 'array') && !props.field.children) {
    props.field.children = []
  }
  if (type === 'string' && !props.field.enum) {
    props.field.enum = []
  }
}

function addEnumValue() {
  if (newEnumVal.value && !props.field.enum.includes(newEnumVal.value)) {
    if (!props.field.enum) props.field.enum = []
    props.field.enum.push(newEnumVal.value)
    newEnumVal.value = ''
  }
}
</script>

<style scoped>
.field-row {
  border-left: 2px solid #ebeef5;
  padding: 8px 0;
}

.field-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.enum-editor {
  margin-top: 8px;
  padding: 8px;
  background: #f5f7fa;
  border-radius: 4px;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.children {
  margin-top: 4px;
}
</style>
