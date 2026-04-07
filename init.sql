-- 银龄健身数据库初始化脚本

-- 肌肉部位分组表
CREATE TABLE IF NOT EXISTS `muscle_groups` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(32) NOT NULL COMMENT '名称',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  `image_url` varchar(255) DEFAULT NULL COMMENT '图片URL',
  `sort` int DEFAULT 0 COMMENT '排序',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_muscle_groups_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='肌肉部位分组表';

-- 肌肉表
CREATE TABLE IF NOT EXISTS `muscles` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `muscle_group_id` bigint unsigned DEFAULT NULL COMMENT '所属分组ID',
  `name` varchar(32) NOT NULL COMMENT '肌肉名称',
  `description` varchar(255) DEFAULT NULL COMMENT '描述',
  `image_url` varchar(255) DEFAULT NULL COMMENT '图片URL',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_muscles_deleted_at` (`deleted_at`),
  KEY `idx_muscles_muscle_group_id` (`muscle_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='肌肉表';

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `openid` varchar(64) NOT NULL COMMENT '微信openid',
  `unionid` varchar(64) DEFAULT NULL COMMENT '微信unionid',
  `nick_name` varchar(64) DEFAULT NULL COMMENT '昵称',
  `avatar_url` varchar(255) DEFAULT NULL COMMENT '头像URL',
  `gender` int DEFAULT 0 COMMENT '性别 0未知 1男 2女',
  `age` int DEFAULT NULL COMMENT '年龄',
  `phone` varchar(20) DEFAULT NULL COMMENT '手机号',
  `health_status` text COMMENT '健康状况',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_openid` (`openid`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 锻炼动作表
CREATE TABLE IF NOT EXISTS `exercises` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `muscle_group_id` bigint unsigned DEFAULT NULL COMMENT '肌肉分组ID',
  `name` varchar(64) NOT NULL COMMENT '动作名称',
  `target_muscle` varchar(255) DEFAULT NULL COMMENT '目标肌肉',
  `description` text COMMENT '描述',
  `difficulty` varchar(16) DEFAULT '简单' COMMENT '难度',
  `duration` int DEFAULT 0 COMMENT '时长(分钟)',
  `sets` int DEFAULT 3 COMMENT '组数',
  `reps` varchar(32) DEFAULT NULL COMMENT '次数',
  `calories` int DEFAULT 0 COMMENT '消耗热量',
  `image_url` varchar(255) DEFAULT NULL COMMENT '图片URL',
  `video_url` varchar(255) DEFAULT NULL COMMENT '视频URL',
  `sort` int DEFAULT 0 COMMENT '排序',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_exercises_deleted_at` (`deleted_at`),
  KEY `idx_exercises_muscle_group_id` (`muscle_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='锻炼动作表';

-- 动作步骤表
CREATE TABLE IF NOT EXISTS `exercise_steps` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `exercise_id` bigint unsigned DEFAULT NULL COMMENT '锻炼ID',
  `order` int DEFAULT 0 COMMENT '步骤顺序',
  `title` varchar(64) DEFAULT NULL COMMENT '标题',
  `desc` text COMMENT '描述',
  `image_url` varchar(255) DEFAULT NULL COMMENT '图片URL',
  `duration` int DEFAULT 0 COMMENT '时长(秒)',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_exercise_steps_deleted_at` (`deleted_at`),
  KEY `idx_exercise_steps_exercise_id` (`exercise_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='动作步骤表';

-- 动作益处表
CREATE TABLE IF NOT EXISTS `exercise_benefits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `exercise_id` bigint unsigned DEFAULT NULL COMMENT '锻炼ID',
  `content` varchar(255) DEFAULT NULL COMMENT '内容',
  `sort` int DEFAULT 0 COMMENT '排序',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_exercise_benefits_deleted_at` (`deleted_at`),
  KEY `idx_exercise_benefits_exercise_id` (`exercise_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='动作益处表';

-- 注意事项表
CREATE TABLE IF NOT EXISTS `exercise_precautions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `exercise_id` bigint unsigned DEFAULT NULL COMMENT '锻炼ID',
  `content` varchar(255) DEFAULT NULL COMMENT '内容',
  `sort` int DEFAULT 0 COMMENT '排序',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_exercise_precautions_deleted_at` (`deleted_at`),
  KEY `idx_exercise_precautions_exercise_id` (`exercise_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='注意事项表';

-- 用户锻炼记录表
CREATE TABLE IF NOT EXISTS `user_exercise_records` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `exercise_id` bigint unsigned DEFAULT NULL COMMENT '锻炼ID',
  `duration` int DEFAULT 0 COMMENT '时长(秒)',
  `sets` int DEFAULT 0 COMMENT '完成组数',
  `completed_at` datetime(3) DEFAULT NULL COMMENT '完成时间',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_exercise_records_deleted_at` (`deleted_at`),
  KEY `idx_user_exercise_records_user_id` (`user_id`),
  KEY `idx_user_exercise_records_exercise_id` (`exercise_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户锻炼记录表';

-- 用户每日统计表
CREATE TABLE IF NOT EXISTS `user_daily_stats` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned DEFAULT NULL COMMENT '用户ID',
  `date` date DEFAULT NULL COMMENT '日期',
  `exercise_count` int DEFAULT 0 COMMENT '锻炼次数',
  `total_duration` int DEFAULT 0 COMMENT '总时长(秒)',
  `total_calories` int DEFAULT 0 COMMENT '总消耗热量',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_user_daily_stats_deleted_at` (`deleted_at`),
  UNIQUE KEY `idx_user_date` (`user_id`, `date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户每日统计表';

-- ============================================
-- 初始数据
-- ============================================

-- 肌肉部位分组
INSERT INTO `muscle_groups` (`name`, `description`, `sort`, `created_at`, `updated_at`) VALUES
('上肢', '增强上肢力量，提高日常活动能力', 1, NOW(), NOW()),
('核心', '稳定身体重心，改善平衡能力', 2, NOW(), NOW()),
('下肢', '增强行走能力，预防跌倒', 3, NOW(), NOW());

-- 上肢锻炼动作
INSERT INTO `exercises` (`muscle_group_id`, `name`, `target_muscle`, `description`, `difficulty`, `duration`, `sets`, `reps`, `calories`, `sort`, `created_at`, `updated_at`) VALUES
(1, '手臂环绕', '肩部', '手臂环绕是一个简单有效的肩部锻炼动作，可以增强肩关节灵活性，预防肩周炎。', '简单', 5, 3, '10-15次', 20, 1, NOW(), NOW()),
(1, '坐姿推举', '肩部、手臂', '坐姿推举可以锻炼肩部和手臂肌肉，增强上肢力量。', '中等', 8, 3, '8-12次', 35, 2, NOW(), NOW()),
(1, '弹力带弯举', '手臂前侧（肱二头肌）', '使用弹力带进行弯举练习，安全有效地增强手臂力量。', '简单', 6, 3, '10-15次', 25, 3, NOW(), NOW()),
(1, '墙壁俯卧撑', '胸部、肩部、手臂', '墙壁俯卧撑是俯卧撑的简化版本，适合老年人练习。', '简单', 5, 3, '8-12次', 30, 4, NOW(), NOW());

-- 核心锻炼动作
INSERT INTO `exercises` (`muscle_group_id`, `name`, `target_muscle`, `description`, `difficulty`, `duration`, `sets`, `reps`, `calories`, `sort`, `created_at`, `updated_at`) VALUES
(2, '坐姿扭转', '腹部、腰部', '坐姿扭转可以锻炼腹部和腰部肌肉，改善脊柱灵活性。', '简单', 5, 3, '10-15次/侧', 20, 1, NOW(), NOW()),
(2, '平板支撑', '核心肌群', '平板支撑是锻炼核心肌群的有效动作，可以增强腹部和背部力量。', '中等', 3, 3, '20-30秒', 30, 2, NOW(), NOW()),
(2, '桥式运动', '臀部、腰部', '桥式运动可以锻炼臀部和腰部肌肉，改善腰部稳定性。', '简单', 6, 3, '10-15次', 25, 3, NOW(), NOW()),
(2, '猫牛式伸展', '背部、腹部', '猫牛式伸展可以增加脊柱灵活性，缓解腰背不适。', '简单', 5, 3, '8-10次', 20, 4, NOW(), NOW());

-- 下肢锻炼动作
INSERT INTO `exercises` (`muscle_group_id`, `name`, `target_muscle`, `description`, `difficulty`, `duration`, `sets`, `reps`, `calories`, `sort`, `created_at`, `updated_at`) VALUES
(3, '坐姿抬腿', '大腿前侧（股四头肌）', '坐姿抬腿是适合老年人的下肢锻炼动作，可以增强大腿肌肉力量。', '简单', 8, 3, '10-15次/腿', 30, 1, NOW(), NOW()),
(3, '靠墙静蹲', '大腿、臀部', '靠墙静蹲可以锻炼大腿和臀部肌肉，增强膝关节稳定性。', '中等', 5, 3, '20-30秒', 35, 2, NOW(), NOW()),
(3, '提踵练习', '小腿', '提踵练习可以增强小腿肌肉力量，提高行走稳定性。', '简单', 5, 3, '15-20次', 20, 3, NOW(), NOW()),
(3, '侧卧抬腿', '臀部外侧、大腿外侧', '侧卧抬腿可以锻炼臀部和大腿外侧肌肉，改善髋关节稳定性。', '简单', 6, 3, '10-15次/侧', 25, 4, NOW(), NOW());

-- 动作步骤 - 坐姿抬腿
INSERT INTO `exercise_steps` (`exercise_id`, `order`, `title`, `desc`, `duration`, `created_at`, `updated_at`) VALUES
(9, 1, '准备姿势', '坐在稳固的椅子上，背部挺直，双手扶住椅子两侧，双脚平放地面。', 10, NOW(), NOW()),
(9, 2, '抬起右腿', '吸气，慢慢将右腿向前抬起，直到与地面平行，膝盖保持伸直。', 5, NOW(), NOW()),
(9, 3, '保持姿势', '在最高点保持2-3秒，感受大腿前侧肌肉收缩。', 3, NOW(), NOW()),
(9, 4, '缓慢放下', '呼气，控制速度慢慢将腿放下，不要突然松劲。', 5, NOW(), NOW()),
(9, 5, '换腿重复', '左腿重复相同动作，完成一组后休息30秒。', 5, NOW(), NOW());

-- 动作益处 - 坐姿抬腿
INSERT INTO `exercise_benefits` (`exercise_id`, `content`, `sort`, `created_at`, `updated_at`) VALUES
(9, '增强大腿肌肉力量', 1, NOW(), NOW()),
(9, '改善膝关节稳定性', 2, NOW(), NOW()),
(9, '提高行走能力', 3, NOW(), NOW()),
(9, '预防跌倒', 4, NOW(), NOW());

-- 注意事项 - 坐姿抬腿
INSERT INTO `exercise_precautions` (`exercise_id`, `content`, `sort`, `created_at`, `updated_at`) VALUES
(9, '动作要缓慢，不要急躁', 1, NOW(), NOW()),
(9, '保持呼吸均匀，不要憋气', 2, NOW(), NOW()),
(9, '如果感到不适请立即停止', 3, NOW(), NOW()),
(9, '膝关节疼痛者应减少抬腿幅度', 4, NOW(), NOW());

-- 动作步骤 - 靠墙静蹲
INSERT INTO `exercise_steps` (`exercise_id`, `order`, `title`, `desc`, `duration`, `created_at`, `updated_at`) VALUES
(10, 1, '准备姿势', '背靠墙壁站立，双脚与肩同宽，脚尖向前，距离墙壁约一步距离。', 10, NOW(), NOW()),
(10, 2, '下蹲动作', '背部贴墙，慢慢下蹲，直到大腿与地面平行（或根据能力调整）。', 5, NOW(), NOW()),
(10, 3, '保持姿势', '保持下蹲姿势20-30秒，膝盖不要超过脚尖，背部紧贴墙壁。', 30, NOW(), NOW()),
(10, 4, '恢复站立', '慢慢沿墙壁向上滑动，恢复站立姿势。', 5, NOW(), NOW()),
(10, 5, '休息重复', '休息30秒后重复下一组。', 30, NOW(), NOW());

-- 动作益处 - 靠墙静蹲
INSERT INTO `exercise_benefits` (`exercise_id`, `content`, `sort`, `created_at`, `updated_at`) VALUES
(10, '增强大腿和臀部肌肉', 1, NOW(), NOW()),
(10, '提高膝关节稳定性', 2, NOW(), NOW()),
(10, '预防膝关节损伤', 3, NOW(), NOW()),
(10, '改善站立平衡', 4, NOW(), NOW());

-- 注意事项 - 靠墙静蹲
INSERT INTO `exercise_precautions` (`exercise_id`, `content`, `sort`, `created_at`, `updated_at`) VALUES
(10, '膝盖不要超过脚尖', 1, NOW(), NOW()),
(10, '背部全程贴紧墙壁', 2, NOW(), NOW()),
(10, '膝关节疼痛者减少下蹲深度', 3, NOW(), NOW()),
(10, '如感头晕立即停止', 4, NOW(), NOW());
