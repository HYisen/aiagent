# GORM [GEN vs CLI](https://gorm.io/cli/cli_vs_gen.html)

## Status

Both of them have been practiced in this project.

I am still evaluating them.

## Goals

What I want is typed.

Then, generic SQL. Even though I don't need it now, I do value smooth transits between SQL dialects.

## Issues

As [it](https://gorm.io/cli/#GORM-CLI-Overview) defines, GORM CLI generates helpers for **common** model operations,
which means occasionally there must be other approach on **less common** operations.

If I fall back to GORM, then I lost something that GORM CLI is better than GORM.

If I switch to GEN, then I miss something that GORM GEN has same to GORM CLI.

Neither a stand-alone struct nor field name string literal shall I accepted to do a partial field select.

## Policy

If GORM CLI could work gracefully, I would choose it first at that task.

Comparing to combining with GORM vanilla without typed or with string literal,
I would rather use GORM GEN alone as long as it's more elegant in that task.
